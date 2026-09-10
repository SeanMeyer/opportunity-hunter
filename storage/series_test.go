package storage_test

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"github.com/seanmeyer/opportunity-hunter/storage"
	"testing"
	"time"
)

func coreFeedback(id int64, rating string) storage.FeedbackRow {
	return storage.FeedbackRow{OpportunityID: &id, HuntName: "comedy", Title: "Sam Jay", Rating: rating, CreatedAt: time.Now()}
}

func TestReconcileComedySeriesPreservesHistoryAndUnprovenDates(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	makeOpp := func(sourceID, date string) int64 {
		start, _ := time.Parse(time.RFC3339, date)
		id, err := db.InsertOpportunity(ctx, core.Opportunity{HuntName: "comedy", Source: "ticketmaster", SourceID: sourceID, Title: "Sam Jay", StartTime: start, ShowDates: []time.Time{start, start.Add(24 * time.Hour)}, State: core.Evaluated, DiscoveredAt: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	old := makeOpp("stable", "2026-11-20T19:00:00-07:00")
	newer := makeOpp("stable", "2027-01-22T19:00:00-07:00")
	db.SaveFeedback(ctx, coreFeedback(old, "up"))
	db.SaveFeedback(ctx, coreFeedback(newer, "down"))
	eval, err := db.SaveEvaluation(ctx, core.Evaluation{HuntName: "comedy", EvaluatedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SavePick(ctx, core.Pick{EvaluationID: eval, OpportunityID: newer, Reason: "current assessment"}); err != nil {
		t.Fatal(err)
	}
	if err := db.ReconcileComedySeries(ctx); err != nil {
		t.Fatal(err)
	}
	active, err := db.GetByState(ctx, "comedy", core.Evaluated)
	if err != nil || len(active) != 1 {
		t.Fatalf("active=%v err=%v", active, err)
	}
	if active[0].ID != old || len(active[0].ShowDates) != 4 {
		t.Fatalf("canonical schedule: %+v", active[0])
	}
	alias, err := db.GetOpportunity(ctx, newer)
	if err != nil || alias.SupersededBy == nil || *alias.SupersededBy != old {
		t.Fatalf("alias=%+v err=%v", alias, err)
	}
	feedback, err := db.GetRecentFeedback(ctx, "comedy", 10)
	if err != nil || len(feedback) != 1 || feedback[0].Rating != "down" || *feedback[0].OpportunityID != old {
		t.Fatalf("feedback=%+v err=%v", feedback, err)
	}
	var count int
	db.RawDB().QueryRow(`SELECT count(*) FROM feedback`).Scan(&count)
	if count != 2 {
		t.Fatal("history lost")
	}
	picks, err := db.GetPicksForOpportunity(ctx, old)
	if err != nil || len(picks) != 1 || picks[0].OpportunityID != newer {
		t.Fatalf("alias evaluation lost: %+v %v", picks, err)
	}
	if _, err := db.SaveFeedback(ctx, coreFeedback(newer, "up")); err != nil {
		t.Fatal(err)
	}
	feedback, err = db.GetRecentFeedback(ctx, "comedy", 10)
	if err != nil || len(feedback) != 1 || feedback[0].Rating != "up" || *feedback[0].OpportunityID != old {
		t.Fatalf("alias feedback did not resolve: %+v %v", feedback, err)
	}
	if err := db.ReconcileComedySeries(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestUpsertComedySeriesExtendsWithoutDuplicateAndKeepsPartialDates(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	venue, err := db.UpsertVenue(ctx, core.Venue{Name: "Comedy Works Downtown", Address: "1226 15th St"})
	if err != nil {
		t.Fatal(err)
	}
	at, _ := time.Parse(time.RFC3339, "2027-01-22T19:00:00-07:00")
	initial := core.Opportunity{HuntName: "comedy", Title: "Example", Source: "test", SourceID: "one", VenueID: &venue, StartTime: at, ShowDates: []time.Time{at, at.Add(24 * time.Hour)}, State: core.Evaluated}
	id, err := db.InsertOpportunity(ctx, initial)
	if err != nil {
		t.Fatal(err)
	}
	incoming := initial
	incoming.SourceID = "two"
	incoming.StartTime = at.Add(24 * time.Hour)
	incoming.ShowDates = []time.Time{at.Add(24 * time.Hour), at.Add(48 * time.Hour)}
	for i := 0; i < 2; i++ {
		created, err := db.UpsertComedySeries(ctx, incoming)
		if err != nil || created {
			t.Fatalf("upsert created=%v err=%v", created, err)
		}
	}
	active, err := db.GetByState(ctx, "comedy", core.Evaluated)
	if err != nil || len(active) != 1 || active[0].ID != id || len(active[0].ShowDates) != 3 {
		t.Fatalf("active=%+v err=%v", active, err)
	}
	var count int
	db.RawDB().QueryRow(`SELECT count(*) FROM opportunities`).Scan(&count)
	if count != 2 {
		t.Fatalf("repeat scan grew aliases: %d", count)
	}
	incoming.ShowDates = []time.Time{at.Add(24 * time.Hour)}
	if _, err := db.UpsertComedySeries(ctx, incoming); err != nil {
		t.Fatal(err)
	}
	o, _ := db.GetOpportunity(ctx, id)
	if len(o.ShowDates) != 3 {
		t.Fatalf("partial source erased dates: %+v", o)
	}
}

func TestUpsertComedySingleEventReschedulesWithoutChangingCardID(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	at, _ := time.Parse(time.RFC3339, "2026-11-20T19:00:00-07:00")
	o := core.Opportunity{HuntName: "comedy", Source: "test", SourceID: "one", Title: "Example", StartTime: at, State: core.Evaluated}
	id, err := db.InsertOpportunity(ctx, o)
	if err != nil {
		t.Fatal(err)
	}
	o.StartTime = at.Add(60 * 24 * time.Hour)
	created, err := db.UpsertComedySeries(ctx, o)
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	saved, err := db.GetOpportunity(ctx, id)
	if err != nil || !saved.StartTime.Equal(o.StartTime) || len(saved.ShowDates) != 1 {
		t.Fatalf("saved=%+v err=%v", saved, err)
	}
}

func TestComedySeriesVenueAndLocalDates(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	insert := func(title, venue, address, date string) int64 {
		v, err := db.UpsertVenue(ctx, core.Venue{Name: venue, Address: address})
		if err != nil {
			t.Fatal(err)
		}
		at, _ := time.Parse(time.RFC3339, date)
		id, err := db.InsertOpportunity(ctx, core.Opportunity{HuntName: "comedy", Title: title, VenueID: &v, StartTime: at, State: core.Evaluated})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	old := insert("Big Jay Oakerson", "Comedy Works", "1226 15th Street", "2026-10-16T02:00:00Z")
	insert("Big Jay Oakerson", "Comedy Works Downtown", "1226 15th St", "2026-10-15T00:00:00-06:00")
	insert("Big Jay Oakerson", "Comedy Works South", "5345 Landmark Pl", "2026-10-15T00:00:00-06:00")
	insert("Big Jay Oakerson presents a special show", "Comedy Works Downtown", "1226 15th St", "2026-10-15T00:00:00-06:00")
	if err := db.ReconcileComedySeries(ctx); err != nil {
		t.Fatal(err)
	}
	active, err := db.GetByState(ctx, "comedy", core.Evaluated)
	if err != nil || len(active) != 3 {
		t.Fatalf("active=%+v err=%v", active, err)
	}
	o, _ := db.GetOpportunity(ctx, old)
	if o.StartTime.Hour() != 2 || len(o.ShowDates) != 1 {
		t.Fatalf("must prefer actual time over date placeholder: %+v", o)
	}
}

func TestMergedComedySourceReschedulePreservesOtherProviderDate(t *testing.T) {
	for _, reconcile := range []bool{false, true} {
		t.Run(map[bool]string{false: "upsert", true: "reconcile"}[reconcile], func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()
			venue, err := db.UpsertVenue(ctx, core.Venue{Name: "Example Venue", Address: "123 Main St"})
			if err != nil {
				t.Fatal(err)
			}
			at := time.Date(2027, 10, 1, 19, 0, 0, 0, time.UTC)
			a := core.Opportunity{HuntName: "comedy", Source: "provider", SourceID: "A", Title: "Example", VenueID: &venue, StartTime: at, State: core.Evaluated}
			id, err := db.InsertOpportunity(ctx, a)
			if err != nil {
				t.Fatal(err)
			}
			b := a
			b.SourceID = "B"
			if _, err := db.UpsertComedySeries(ctx, b); err != nil {
				t.Fatal(err)
			}
			a.StartTime = at.Add(7 * 24 * time.Hour)
			if reconcile {
				if _, err := db.InsertOpportunity(ctx, a); err != nil {
					t.Fatal(err)
				}
				if err := db.ReconcileComedySeries(ctx); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := db.UpsertComedySeries(ctx, a); err != nil {
					t.Fatal(err)
				}
			}
			saved, err := db.GetOpportunity(ctx, id)
			if err != nil || len(saved.ShowDates) != 2 || !saved.ShowDates[0].Equal(at) || !saved.ShowDates[1].Equal(a.StartTime) {
				t.Fatalf("other provider date lost: %+v %v", saved, err)
			}
		})
	}
}

func TestComedyReconcileConnectedObservationsInOnePass(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(map[bool]string{false: "forward", true: "reverse"}[reverse], func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()
			venue, err := db.UpsertVenue(ctx, core.Venue{Name: "Example Venue", Address: "123 Main St"})
			if err != nil {
				t.Fatal(err)
			}
			at := time.Date(2027, 10, 1, 19, 0, 0, 0, time.UTC)
			next := at.Add(7 * 24 * time.Hour)
			observations := [][]time.Time{{at}, {at, next}, {next}}
			if reverse {
				observations[0], observations[2] = observations[2], observations[0]
			}
			var ids []int64
			for _, dates := range observations {
				id, err := db.InsertOpportunity(ctx, core.Opportunity{HuntName: "comedy", Title: "Example", VenueID: &venue, StartTime: dates[0], ShowDates: dates, State: core.Evaluated})
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
			}
			for run := 0; run < 2; run++ {
				if err := db.ReconcileComedySeries(ctx); err != nil {
					t.Fatal(err)
				}
				active, err := db.GetByState(ctx, "comedy", core.Evaluated)
				if err != nil || len(active) != 1 || active[0].ID != ids[0] || len(active[0].ShowDates) != 2 {
					t.Fatalf("pass%d active=%+v err=%v", run, active, err)
				}
				for _, id := range ids[1:] {
					alias, err := db.GetOpportunity(ctx, id)
					if err != nil || alias.SupersededBy == nil || *alias.SupersededBy != ids[0] {
						t.Fatalf("pass%d alias=%+v err=%v", run, alias, err)
					}
				}
			}
		})
	}
}

func TestComedyPartialObservationCannotEraseMergedHistory(t *testing.T) {
	for _, priorAlias := range []bool{false, true} {
		t.Run(map[bool]string{false: "component", true: "prior alias"}[priorAlias], func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()
			at := time.Date(2027, 10, 1, 19, 0, 0, 0, time.UTC)
			a := core.Opportunity{HuntName: "comedy", Source: "provider", SourceID: "A", Title: "Example", StartTime: at, State: core.Evaluated}
			id, err := db.InsertOpportunity(ctx, a)
			if err != nil {
				t.Fatal(err)
			}
			b := a
			b.ShowDates = []time.Time{at, at.Add(7 * 24 * time.Hour)}
			bid, err := db.InsertOpportunity(ctx, b)
			if err != nil {
				t.Fatal(err)
			}
			if priorAlias {
				if _, err := db.RawDB().Exec(`UPDATE opportunities SET superseded_by=? WHERE id=?`, id, bid); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.InsertOpportunity(ctx, a); err != nil {
				t.Fatal(err)
			}
			if err := db.ReconcileComedySeries(ctx); err != nil {
				t.Fatal(err)
			}
			o, err := db.GetOpportunity(ctx, id)
			// Prior aliases are preserved as history rather than added back to the
			// active schedule. They still invalidate proof of single-event ownership.
			if !priorAlias && (err != nil || len(o.ShowDates) != 2) {
				t.Fatalf("partial observation erased component dates: %+v %v", o, err)
			}
			if priorAlias {
				a.StartTime = at.Add(14 * 24 * time.Hour)
				if _, err := db.UpsertComedySeries(ctx, a); err != nil {
					t.Fatal(err)
				}
				o, err = db.GetOpportunity(ctx, id)
				if err != nil || len(o.ShowDates) != 2 || !o.ShowDates[0].Equal(at) {
					t.Fatalf("prior multi-date identity treated as singleton: %+v %v", o, err)
				}
			}
		})
	}
}
