package pipeline

import (
	"context"
	"errors"
	"fmt"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"slices"
	"sort"
	"time"
)

// Delivery intents survive restarts. Actions are frozen before contacting the
// notifier so a retry does not re-evaluate or change the promised message.
// Returns false after a send failure so later passes in this hunt run are skipped.
func (p *Pipeline) deliverPending(ctx context.Context, hunt core.Hunt, caps core.HuntCapabilities, synthesis map[string]string, attempted map[int64]bool, result *core.HuntResult) bool {
	if p.dryRun {
		return true
	}
	pending, err := p.db.PendingDeliveries(ctx, hunt.Name())
	addErr := func(err error) { result.Errors = append(result.Errors, core.StepError{Step: "notify", Err: err}) }
	if err != nil {
		addErr(err)
		return true
	}
	for _, delivery := range pending {
		if attempted[delivery.EvaluationID] {
			continue
		}
		attempted[delivery.EvaluationID] = true
		// An explicit opt-out consumes queued work without preparing or sending it.
		// Dry runs return above so they continue to preserve the queue.
		if notifier, ok := p.notifier.(interface{ NotificationsEnabled(string) bool }); ok && !notifier.NotificationsEnabled(hunt.Name()) {
			if err := p.db.CompleteDelivery(ctx, delivery.EvaluationID); err != nil {
				addErr(err)
			}
			continue
		}
		// Persisted delivery payloads can outlive their event, including actions
		// prepared by an earlier version. Consult current state before any send.
		expirer, _ := hunt.(core.Expirer)
		valid := make(map[int64]bool)
		var current []core.Opportunity
		obsolete := false
		scheduleChanged := false
		readFailed := false
		for _, saved := range delivery.Context.Opportunities {
			opp, err := p.db.GetOpportunity(ctx, saved.ID)
			if err != nil {
				addErr(err)
				readFailed = true
				break
			}
			if opp.SupersededBy != nil || core.OpportunityExpired(opp, expirer, time.Now()) {
				obsolete = true
				continue
			}
			valid[opp.ID] = true
			upcoming := core.UpcomingListing(opp, time.Now())
			if !sameDeliverySchedule(saved, upcoming) {
				scheduleChanged = true
			}
			current = append(current, upcoming)
		}
		if readFailed {
			continue
		}
		if scheduleChanged {
			// Even an unprepared pick can contain old dates in its prose. Frozen
			// actions may also have acknowledged predecessors, so neither form is
			// safe to regenerate or send after the remaining schedule changes.
			addErr(fmt.Errorf("delivery %d schedule changed since evaluation; queued work retained without sending", delivery.EvaluationID))
			continue
		}
		if obsolete {
			if len(current) == 0 {
				if err := p.db.CompleteDelivery(ctx, delivery.EvaluationID); err != nil {
					addErr(err)
				}
				continue
			}
			if delivery.Prepared {
				// Frozen actions lack per-item attribution. Regenerating them may
				// replay acknowledged sends; retain the remainder for inspection.
				addErr(fmt.Errorf("delivery %d contains expired and current opportunities; prepared actions retained without sending", delivery.EvaluationID))
				continue
			}
			picks := make([]core.Pick, 0, len(delivery.Context.Picks))
			for _, pick := range delivery.Context.Picks {
				if valid[pick.OpportunityID] {
					picks = append(picks, pick)
				}
			}
			delivery.Context.Picks = picks
			// Group summaries may mention removed opportunities.
			delivery.Context.Synthesis = ""
			delivery.Context.Evaluations = nil
		}
		delivery.Context.Opportunities = current
		if !delivery.Prepared {
			var actions []core.NotifyAction
			if caps.HasNotifyHunt {
				if formatter := hunt.(core.NotifyHunt).NotifyFormatter(); formatter != nil {
					thread, err := p.db.GetThread(ctx, hunt.Name(), delivery.GroupKey)
					if err != nil && !errors.Is(err, storage.ErrNotFound) {
						addErr(err)
						continue
					}
					delivery.Context.ExistingThreadID = thread
					if text, ok := synthesis[delivery.GroupKey]; ok && delivery.Context.Synthesis == "" && !obsolete {
						delivery.Context.Synthesis = text
					}
					actions = formatter.FormatPicks(delivery.Context)
				}
			}
			if err := p.db.PrepareDeliveryContext(ctx, delivery.EvaluationID, delivery.Context, actions); err != nil {
				addErr(err)
				continue
			}
			delivery.Actions = actions
		}
		if err := core.ValidateActions(delivery.Actions); err != nil {
			addErr(err)
			continue
		}
		if len(delivery.Actions) == 0 {
			if err := p.db.CompleteDelivery(ctx, delivery.EvaluationID); err != nil {
				addErr(err)
			}
			continue
		}
		for len(delivery.Actions) > 0 {
			action := delivery.Actions[0]
			threads, sendErr := p.notifier.ExecuteActions(hunt.Name(), []core.NotifyAction{action})
			// A returned create ID is a known success even if the notifier also
			// reports an error. Never discard that acknowledgment and recreate it.
			knownCreate := action.Type == core.CreateThread && threads[action.ThreadName] != ""
			if sendErr != nil && !knownCreate {
				addErr(sendErr)
				return false
			}
			if action.Type == core.CreateThread && !knownCreate {
				addErr(fmt.Errorf("created thread %q returned no ID", action.ThreadName))
				break
			}
			remaining := append([]core.NotifyAction(nil), delivery.Actions[1:]...)
			for i := range remaining {
				if id := threads[remaining[i].ThreadRef]; remaining[i].ThreadRef != "" && id != "" {
					remaining[i].ThreadID = id
					remaining[i].ThreadRef = ""
				}
			}
			if err := p.db.CheckpointDelivery(ctx, delivery.EvaluationID, hunt.Name(), delivery.GroupKey, remaining, threads); err != nil {
				addErr(err)
				if sendErr != nil {
					addErr(sendErr)
					return false
				}
				break
			}
			delivery.Actions = remaining
			result.Notified++
			if sendErr != nil {
				addErr(sendErr)
				return false
			}
		}
	}
	return true
}

func sameDeliverySchedule(a, b core.Opportunity) bool {
	// SQLite stores timestamps at second precision. Ignore source order and
	// timezone representation, but do not drop elapsed dates from saved work.
	key := func(at time.Time) string { return at.UTC().Format(time.RFC3339) }
	if key(a.StartTime) != key(b.StartTime) || (a.EndTime == nil) != (b.EndTime == nil) {
		return false
	}
	if a.EndTime != nil && key(*a.EndTime) != key(*b.EndTime) {
		return false
	}
	dates := func(values []time.Time) []string {
		out := make([]string, 0, len(values))
		for _, at := range values {
			out = append(out, key(at))
		}
		sort.Strings(out)
		return out
	}
	return slices.Equal(dates(a.ShowDates), dates(b.ShowDates))
}
