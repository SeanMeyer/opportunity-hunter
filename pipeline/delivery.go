package pipeline

import (
	"context"
	"errors"
	"fmt"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/storage"
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
					if text, ok := synthesis[delivery.GroupKey]; ok && delivery.Context.Synthesis == "" {
						delivery.Context.Synthesis = text
					}
					actions = formatter.FormatPicks(delivery.Context)
				}
			}
			if err := p.db.PrepareDelivery(ctx, delivery.EvaluationID, actions); err != nil {
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
