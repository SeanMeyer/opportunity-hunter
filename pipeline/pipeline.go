package pipeline

import (
	"context"
	"log/slog"
	"sort"
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
	homeRegion  core.ScanRegion
	homeAddress string
}

// Notifier abstracts Discord notification sending.
// The hunt name is passed so implementations can route to per-hunt webhooks.
type Notifier interface {
	ExecuteActions(huntName string, actions []core.NotifyAction) (map[string]string, error)
	PostError(message string) error
}

// New creates a pipeline.
func New(db *storage.DB, costTracker *core.CostTracker, notifier Notifier, homeRegion core.ScanRegion, homeAddress string) *Pipeline {
	return &Pipeline{
		db:          db,
		costTracker: costTracker,
		notifier:    notifier,
		homeRegion:  homeRegion,
		homeAddress: homeAddress,
	}
}

// ScanAll runs only the scan step for all hunts (no evaluation, no notifications).
func (p *Pipeline) ScanAll(ctx context.Context, hunts []core.Hunt) core.PipelineResult {
	var result core.PipelineResult
	for _, hunt := range hunts {
		name := hunt.Name()
		hr := core.HuntResult{HuntName: name}
		caps, _ := core.ValidateHunt(hunt)
		scanned, scanErrs := p.scan(ctx, hunt)
		hr.Scanned = scanned
		hr.Errors = append(hr.Errors, scanErrs...)
		p.expire(ctx, hunt, caps)
		result.HuntResults = append(result.HuntResults, hr)
	}
	return result
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

	// Record the run start; inject run ID into context for log association.
	trigger := storage.TriggerFromContext(ctx)
	runID, insertErr := p.db.InsertRun(ctx, name, trigger)
	if insertErr != nil {
		slog.Warn("failed to record pipeline run start", "hunt", name, "err", insertErr)
	} else {
		ctx = storage.ContextWithRunID(ctx, runID)
	}

	costBefore := p.costTracker.ForHunt(name)

	defer func() {
		if insertErr != nil {
			return // nothing to finish if we couldn't insert
		}
		status := "ok"
		if len(result.Errors) > 0 {
			status = "error"
		}
		costDelta := p.costTracker.ForHunt(name) - costBefore
		var errSummary string
		if len(result.Errors) > 0 {
			errSummary = result.Errors[0].Err.Error()
		}
		if err := p.db.FinishRun(ctx, runID, storage.RunResult{
			Status:       status,
			Scanned:      result.Scanned,
			Evaluated:    result.Evaluated,
			Notified:     result.Notified,
			CostUSD:      costDelta,
			ErrorSummary: errSummary,
		}); err != nil {
			slog.Warn("failed to finish pipeline run record", "hunt", name, "err", err)
		}
	}()

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
	var allPicks []core.Pick
	var allOpps []core.Opportunity
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
		allPicks = append(allPicks, picks...)
		allOpps = append(allOpps, group.Opportunities...)

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

	// Step 8: Brief — if Briefer, synthesize per notify group.
	var synthesis map[string]string // groupKey → synthesis text
	if caps.HasBriefer && len(evals) > 0 {
		briefer := hunt.(core.Briefer)
		notifyGroups := briefer.GroupForNotify(evals)
		synthesis = make(map[string]string, len(notifyGroups))
		for _, ng := range notifyGroups {
			text, err := briefer.Synthesize(ctx, ng, p.costTracker)
			if err != nil {
				slog.Warn("briefing failed", "hunt", name, "group", ng.Key, "err", err)
				continue
			}
			synthesis[ng.Key] = text
		}
	}

	// Step 9: Notify.
	if !p.dryRun && len(evals) > 0 {
		p.notifyFull(ctx, hunt, caps, evals, allPicks, allOpps, synthesis, &result)
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
			items, err := s.Scan(ctx, p.homeRegion)
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

	// Pre-seed dedup keys from existing DB rows so normalized titles
	// catch variants across scans (e.g., "Club Seating - X" vs "X").
	// For merged multi-date events, expand ShowDates into individual dedup keys
	// so new scan dates that already exist in a merged opp are skipped.
	seen := make(map[string]bool)
	existing, _ := p.db.GetRawItemsForDedup(ctx, name)
	for _, ex := range existing {
		if len(ex.ShowDates) > 0 {
			for _, d := range ex.ShowDates {
				expanded := ex
				expanded.StartTime = d.Format(time.RFC3339)
				seen[hunt.DedupeKey(expanded)] = true
			}
		} else {
			seen[hunt.DedupeKey(ex)] = true
		}
	}

	// Collect best candidate per dedup key. When multiple source items
	// produce the same key (e.g., "Club Seating - X" and "X: Tour Name"),
	// prefer the one whose title is closest to the normalized form —
	// i.e., the "real" event name rather than a venue seating variant.
	best := make(map[string]core.RawItem)
	var keyOrder []string
	for _, item := range allItems {
		key := hunt.DedupeKey(item)
		if seen[key] {
			continue
		}
		if prev, exists := best[key]; exists {
			// Prefer the title that required less normalization (shorter diff).
			prevNorm := core.NormalizeTitle(prev.Title)
			itemNorm := core.NormalizeTitle(item.Title)
			prevDelta := len(prev.Title) - len(prevNorm)
			itemDelta := len(item.Title) - len(itemNorm)
			if itemDelta < prevDelta {
				best[key] = item
			}
		} else {
			best[key] = item
			keyOrder = append(keyOrder, key)
		}
	}

	// Merge pass: group best items by normalized title + venue (no date)
	// to merge multi-date events (e.g., Fri/Sat/Sun shows) into one opportunity.
	merged := mergeMultiDateItems(best, keyOrder)

	stored := 0
	for _, item := range merged {
		for _, key := range item.mergedKeys {
			seen[key] = true
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
			ShowDates:    item.ShowDates,
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

	// Enrich venues (e.g. walking distance) and cache results.
	if enricher, ok := hunt.(core.VenueEnricher); ok {
		enricher.EnrichVenues(ctx, venues)
		for _, v := range venues {
			if v.WalkingMinutes > 0 || v.DistanceMi > 0 {
				p.db.SaveDistance(ctx, storage.DistanceRow{
					VenueID:     v.ID,
					HomeAddress: p.homeAddress,
					Mode:        "walking",
					Minutes:     v.WalkingMinutes,
					DistanceMi:  v.DistanceMi,
					CreatedAt:   time.Now(),
				})
			}
		}
	}

	// Load preferences, seeding defaults if empty.
	prefs, _ := p.db.GetPreferences(ctx, hunt.Name())
	if prefs == "" {
		if dp, ok := hunt.(core.DefaultPreferencer); ok {
			prefs = dp.DefaultPreferences()
			_ = p.db.SavePreferences(ctx, hunt.Name(), prefs)
			slog.Info("seeded default preferences", "hunt", hunt.Name())
		}
	}
	fbRows, _ := p.db.GetRecentFeedback(ctx, hunt.Name(), 20)
	var feedback []core.FeedbackEntry
	for _, fb := range fbRows {
		feedback = append(feedback, core.FeedbackEntry{
			OpportunityTitle: fb.Title,
			Rating:           fb.Rating,
			Note:             fb.Note,
			EvalSummary:      fb.EvalSummary,
			EvalScore:        fb.EvalScore,
		})
	}

	// Load structured profile (hunt-specific with global fallback).
	profile, _ := p.db.GetProfile(ctx, hunt.Name())

	ec := core.EvalContext{
		Opportunities: group.Opportunities,
		Venues:        venues,
		Preferences:   prefs,
		Profile:       profile,
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

func (p *Pipeline) notifyFull(ctx context.Context, hunt core.Hunt, caps core.HuntCapabilities, evals []core.Evaluation, picks []core.Pick, opps []core.Opportunity, synthesis map[string]string, result *core.HuntResult) {
	if !caps.HasNotifyHunt {
		return
	}

	formatter := hunt.(core.NotifyHunt).NotifyFormatter()
	if formatter == nil {
		return
	}

	name := hunt.Name()

	// Look up existing thread for this group.
	groupKey := ""
	if len(evals) > 0 {
		groupKey = evals[0].GroupKey
	}
	existingThread, _ := p.db.GetThread(ctx, name, groupKey)

	// Get synthesis text for this group.
	synthText := ""
	if synthesis != nil {
		synthText = synthesis[groupKey]
	}

	nCtx := core.NotifyContext{
		Evaluations:      evals,
		Picks:            picks,
		Opportunities:    opps,
		Synthesis:        synthText,
		ExistingThreadID: existingThread,
	}
	actions := formatter.FormatPicks(nCtx)
	if len(actions) == 0 {
		return
	}

	threadIDs, err := p.notifier.ExecuteActions(name, actions)
	if err != nil {
		result.Errors = append(result.Errors, core.StepError{Step: "notify", Err: err})
		return
	}
	result.Notified = len(actions)

	// Persist newly created thread IDs.
	for threadName, threadID := range threadIDs {
		if saveErr := p.db.SaveThread(ctx, name, groupKey, threadID); saveErr != nil {
			slog.Warn("failed to save thread", "hunt", name, "thread", threadName, "err", saveErr)
		}
	}
}

// mergedItem is a RawItem extended with merge metadata.
type mergedItem struct {
	core.RawItem
	mergedKeys []string // all dedup keys that were merged into this item
}

// mergeMultiDateItems groups best items by normalized title + venue (no date component),
// merging multi-date events into a single item with all dates in ShowDates.
func mergeMultiDateItems(best map[string]core.RawItem, keyOrder []string) []mergedItem {
	type mergeGroup struct {
		items []core.RawItem
		keys  []string
	}

	groups := make(map[string]*mergeGroup)
	var groupOrder []string

	for _, key := range keyOrder {
		item := best[key]
		mergeKey := core.NormalizeTitleForDedup(item.Title) + "|" + item.VenueName
		if g, ok := groups[mergeKey]; ok {
			g.items = append(g.items, item)
			g.keys = append(g.keys, key)
		} else {
			groups[mergeKey] = &mergeGroup{
				items: []core.RawItem{item},
				keys:  []string{key},
			}
			groupOrder = append(groupOrder, mergeKey)
		}
	}

	result := make([]mergedItem, 0, len(groupOrder))
	for _, mk := range groupOrder {
		g := groups[mk]

		// Sort items by start time.
		sort.Slice(g.items, func(i, j int) bool {
			ti, _ := time.Parse(time.RFC3339, g.items[i].StartTime)
			tj, _ := time.Parse(time.RFC3339, g.items[j].StartTime)
			return ti.Before(tj)
		})

		// Use earliest item as the representative.
		rep := g.items[0]

		// Collect all dates.
		var dates []time.Time
		for _, item := range g.items {
			t, err := time.Parse(time.RFC3339, item.StartTime)
			if err == nil {
				dates = append(dates, t)
			}
		}
		rep.ShowDates = dates

		result = append(result, mergedItem{
			RawItem:    rep,
			mergedKeys: g.keys,
		})
	}
	return result
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
				// Default: expire after the last show date (or StartTime for single-date).
				shouldExpire = opp.LastShowDate().Before(time.Now())
			}
			if shouldExpire {
				p.db.UpdateState(ctx, opp.ID, core.Expired, nil)
			}
		}
	}
}
