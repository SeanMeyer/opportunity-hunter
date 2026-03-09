package pipeline

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/storage"
)

// Pipeline orchestrates the hunt execution loop.
type Pipeline struct {
	db          *storage.DB
	costTracker *core.CostTracker
	notifier    Notifier
	dryRun      bool
}

// Notifier abstracts Discord notification sending.
type Notifier interface {
	ExecuteActions(actions []core.NotifyAction) error
	PostError(message string) error
}

// New creates a pipeline.
func New(db *storage.DB, costTracker *core.CostTracker, notifier Notifier) *Pipeline {
	return &Pipeline{
		db:          db,
		costTracker: costTracker,
		notifier:    notifier,
	}
}

// RunAll runs the pipeline for all hunts sequentially.
func (p *Pipeline) RunAll(ctx context.Context, hunts []core.Hunt) core.PipelineResult {
	var result core.PipelineResult
	for _, hunt := range hunts {
		hr := p.Run(ctx, hunt)
		result.HuntResults = append(result.HuntResults, hr)
	}
	return result
}

// Run executes the full pipeline for a single hunt.
func (p *Pipeline) Run(ctx context.Context, hunt core.Hunt) core.HuntResult {
	name := hunt.Name()
	result := core.HuntResult{HuntName: name}

	caps, _ := core.ValidateHunt(hunt)
	schedule := hunt.DefaultSchedule()

	// Step 1: Scan — fetch from sources concurrently, dedupe, store.
	scanned, scanErrs := p.scan(ctx, hunt)
	result.Scanned = scanned
	result.Errors = append(result.Errors, scanErrs...)

	// Step 2: Collect — get discovered opportunities.
	discovered, err := p.db.GetByState(ctx, name, core.Discovered)
	if err != nil {
		result.Errors = append(result.Errors, core.StepError{Step: "collect", Err: err})
		return result
	}

	// Step 3: Gate — check budget.
	if schedule.MaxMonthlySpendUSD != nil {
		spent := p.costTracker.ForHunt(name)
		if spent >= *schedule.MaxMonthlySpendUSD {
			slog.Info("budget exceeded, skipping evaluation", "hunt", name, "spent", spent, "limit", *schedule.MaxMonthlySpendUSD)
			return result
		}
	}

	// Step 4: ReEval — check for re-evaluation candidates.
	var reEvalItems []core.Opportunity
	if caps.HasReEvaluator {
		reEvalItems = p.collectReEvalCandidates(ctx, hunt.(core.ReEvaluator), name)
	}

	// Merge discovered + re-eval candidates.
	allItems := append(discovered, reEvalItems...)
	if len(allItems) == 0 {
		// Nothing to evaluate — still run expire.
		p.expire(ctx, hunt, caps)
		return result
	}

	// Step 5: Group — batch for evaluation.
	groups := p.groupForEval(hunt, caps, allItems)

	// Step 6 + 7: Evaluate + Store.
	var evals []core.Evaluation
	for _, group := range groups {
		eval, picks, evalErr := p.evaluateGroup(ctx, hunt, group)
		if evalErr != nil {
			result.Errors = append(result.Errors, core.StepError{
				Step:    "evaluate",
				Err:     evalErr,
				Context: group.Key,
			})
			continue
		}
		result.Evaluated++
		evals = append(evals, eval)

		// Store evaluation + picks.
		evalID, storeErr := p.db.SaveEvaluationWithPicks(ctx, eval, picks)
		if storeErr != nil {
			result.Errors = append(result.Errors, core.StepError{Step: "store", Err: storeErr, Context: group.Key})
			continue
		}

		// Update opportunity states.
		now := time.Now()
		for _, opp := range group.Opportunities {
			p.db.UpdateState(ctx, opp.ID, core.Evaluated, &now)
		}

		// Track cost.
		p.costTracker.Add(name, eval.CostUSD)
		p.db.RecordCost(ctx, name, eval.CostUSD, "gemini", true)

		_ = evalID
	}

	// Step 8: Brief — if Briefer, synthesize.
	// (Skipped for now — implemented when powder hunt lands)

	// Step 9: Notify.
	if !p.dryRun && len(evals) > 0 {
		p.notify(ctx, hunt, caps, evals, &result)
	}

	// Step 11: Expire.
	p.expire(ctx, hunt, caps)

	return result
}

// scan fetches from all sources concurrently, deduplicates, and stores new opportunities.
func (p *Pipeline) scan(ctx context.Context, hunt core.Hunt) (int, []core.StepError) {
	sources := hunt.Sources()
	name := hunt.Name()

	type scanResult struct {
		items []core.RawItem
		err   error
		src   string
	}

	var mu sync.Mutex
	var results []scanResult
	var wg sync.WaitGroup

	for _, src := range sources {
		wg.Add(1)
		go func(s core.Source) {
			defer wg.Done()
			items, err := s.Scan(ctx, core.ScanRegion{})
			mu.Lock()
			results = append(results, scanResult{items: items, err: err, src: s.Name()})
			mu.Unlock()
		}(src)
	}
	wg.Wait()

	var errs []core.StepError
	var allItems []core.RawItem
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, core.StepError{Step: "scan", Err: r.err, Context: r.src})
			continue
		}
		allItems = append(allItems, r.items...)
	}

	// Deduplicate and store.
	seen := make(map[string]bool)
	stored := 0
	for _, item := range allItems {
		key := hunt.DedupeKey(item)
		if seen[key] {
			continue
		}
		seen[key] = true

		// Check if already in DB.
		exists, _ := p.db.OpportunityExists(ctx, name, item.SourceID)
		if exists {
			continue
		}

		// Upsert venue if provided.
		var venueID *int64
		if item.VenueName != "" {
			id, err := p.db.UpsertVenue(ctx, core.Venue{
				Name:      item.VenueName,
				Address:   item.VenueAddress,
				Latitude:  item.VenueLatitude,
				Longitude: item.VenueLongitude,
			})
			if err == nil {
				venueID = &id
			}
		}

		// Parse times.
		startTime, _ := time.Parse(time.RFC3339, item.StartTime)
		var endTime *time.Time
		if item.EndTime != "" {
			t, _ := time.Parse(time.RFC3339, item.EndTime)
			endTime = &t
		}

		opp := core.Opportunity{
			HuntName:     name,
			SourceID:     item.SourceID,
			Source:       item.Source,
			Title:        item.Title,
			Subtitle:     item.Subtitle,
			VenueID:      venueID,
			StartTime:    startTime,
			EndTime:      endTime,
			PriceMin:     item.PriceMin,
			PriceMax:     item.PriceMax,
			TicketURL:    item.TicketURL,
			State:        core.Discovered,
			Attributes:   item.Attributes,
			RawData:      item.RawJSON,
			DiscoveredAt: time.Now(),
		}

		if _, err := p.db.InsertOpportunity(ctx, opp); err != nil {
			slog.Error("insert opportunity", "hunt", name, "title", item.Title, "err", err)
			continue
		}
		stored++
	}

	return stored, errs
}

func (p *Pipeline) collectReEvalCandidates(ctx context.Context, re core.ReEvaluator, huntName string) []core.Opportunity {
	// Get evaluated/notified opportunities for re-eval check.
	var candidates []core.Opportunity
	for _, state := range []core.State{core.Evaluated, core.Notified} {
		opps, err := p.db.GetByState(ctx, huntName, state)
		if err != nil {
			continue
		}
		for _, opp := range opps {
			latest, _ := p.db.GetLatestEvaluation(ctx, huntName, opp.Title)
			if re.ShouldReEvaluate(opp, &latest) {
				candidates = append(candidates, opp)
			}
		}
	}
	return candidates
}

func (p *Pipeline) groupForEval(hunt core.Hunt, caps core.HuntCapabilities, items []core.Opportunity) []core.Group {
	if caps.HasGrouper {
		return hunt.(core.Grouper).GroupForEval(items)
	}
	// Default: one group per opportunity.
	groups := make([]core.Group, len(items))
	for i, item := range items {
		groups[i] = core.Group{
			Key:           item.Title,
			Opportunities: []core.Opportunity{item},
		}
	}
	return groups
}

func (p *Pipeline) evaluateGroup(ctx context.Context, hunt core.Hunt, group core.Group) (core.Evaluation, []core.Pick, error) {
	// Build venue map.
	venues := make(map[int64]core.Venue)
	for _, opp := range group.Opportunities {
		if opp.VenueID != nil {
			v, err := p.db.GetVenue(ctx, *opp.VenueID)
			if err == nil {
				venues[v.ID] = v
			}
		}
	}

	// Load preferences and feedback.
	prefs, _ := p.db.GetPreferences(ctx, hunt.Name())
	fbRows, _ := p.db.GetRecentFeedback(ctx, hunt.Name(), 20)
	var feedback []core.FeedbackEntry
	for _, fb := range fbRows {
		feedback = append(feedback, core.FeedbackEntry{
			OpportunityTitle: fb.Title,
			Rating:           fb.Rating,
			Note:             fb.Note,
		})
	}

	ec := core.EvalContext{
		Opportunities: group.Opportunities,
		Venues:        venues,
		Preferences:   prefs,
		Feedback:      feedback,
		CostTracker:   p.costTracker,
	}

	result, err := hunt.Evaluator().Evaluate(ctx, ec)
	if err != nil {
		return core.Evaluation{}, nil, err
	}

	result.Evaluation.HuntName = hunt.Name()
	result.Evaluation.GroupKey = group.Key

	// Set opportunity IDs on picks if not already set.
	for i := range result.Picks {
		if result.Picks[i].OpportunityID == 0 && len(group.Opportunities) > 0 {
			result.Picks[i].OpportunityID = group.Opportunities[0].ID
		}
	}

	return result.Evaluation, result.Picks, nil
}

func (p *Pipeline) notify(ctx context.Context, hunt core.Hunt, caps core.HuntCapabilities, evals []core.Evaluation, result *core.HuntResult) {
	if !caps.HasNotifyHunt {
		return
	}

	formatter := hunt.(core.NotifyHunt).NotifyFormatter()
	if formatter == nil {
		return
	}

	nCtx := core.NotifyContext{Evaluations: evals}
	actions := formatter.FormatPicks(nCtx)
	if len(actions) == 0 {
		return
	}

	if err := p.notifier.ExecuteActions(actions); err != nil {
		result.Errors = append(result.Errors, core.StepError{Step: "notify", Err: err})
		return
	}
	result.Notified = len(actions)
}

func (p *Pipeline) expire(ctx context.Context, hunt core.Hunt, caps core.HuntCapabilities) {
	name := hunt.Name()
	for _, state := range []core.State{core.Notified, core.Reminded, core.Evaluated} {
		opps, err := p.db.GetByState(ctx, name, state)
		if err != nil {
			continue
		}
		for _, opp := range opps {
			shouldExpire := false
			if caps.HasExpirer {
				shouldExpire = hunt.(core.Expirer).ShouldExpire(opp)
			} else {
				// Default: expire if start time is in the past.
				shouldExpire = opp.StartTime.Before(time.Now())
			}
			if shouldExpire {
				p.db.UpdateState(ctx, opp.ID, core.Expired, nil)
			}
		}
	}
}
