package storage

import (
	"context"
	"github.com/seanmeyer/opportunity-hunter/core"
	"testing"
	"time"
)

func TestReviewBackfillAtomicAndIdempotent(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	o := core.Opportunity{HuntName: "comedy", SourceID: "x", Title: "Artist", State: core.Evaluated, StartTime: time.Now().Add(24 * time.Hour), DiscoveredAt: time.Now()}
	id, e := d.InsertOpportunity(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	o.ID = id
	eid, e := d.SaveEvaluationWithPicks(ctx, core.Evaluation{HuntName: "comedy", EvaluatedAt: time.Now()}, []core.Pick{{OpportunityID: id, Score: .8, Reason: "Original reason", Attributes: []byte(`{"sell_out_risk":"low"}`)}})
	if e != nil {
		t.Fatal(e)
	}
	picks, _ := d.GetPicksForOpportunity(ctx, id)
	p := picks[0]
	checkedAt := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	ev := []core.ReviewEvidence{{Kind: "review", Title: "Critic", Subject: "Past special", Summary: "A concise critique.", URL: "https://critic.example/review"}}
	ok, e := d.ApplyReviewBackfill(ctx, o, p, ev, "audit", .1, "test", checkedAt)
	if e != nil || !ok {
		t.Fatalf("apply %v %v", ok, e)
	}
	ok, e = d.ApplyReviewBackfill(ctx, o, p, ev, "audit", .1, "test", checkedAt)
	if e != nil || ok {
		t.Fatalf("repeat %v %v", ok, e)
	}
	got, _ := d.GetPicksForOpportunity(ctx, id)
	if got[0].EvaluationID != eid || got[0].Score != .8 || got[0].Reason != p.Reason || len(core.ReadReviewEvidence(got[0].Attributes)) != 1 {
		t.Fatalf("changed judgment %+v", got)
	}
	var costDate string
	if e := d.db.QueryRow("select evaluated_at from eval_costs").Scan(&costDate); e != nil || costDate != checkedAt.Format(time.RFC3339) {
		t.Fatalf("wrong research cost date %s: %v", costDate, e)
	}
	var count int
	d.db.QueryRow("select count(*) from pending_deliveries").Scan(&count)
	if count != 0 {
		t.Fatal("created delivery")
	}
	d.db.QueryRow("select count(*) from eval_costs").Scan(&count)
	if count != 1 {
		t.Fatal("duplicate costs")
	}
}
func TestReviewBackfillRejectsNewerPick(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	o := core.Opportunity{HuntName: "movies", SourceID: "x", Title: "Film", State: core.Evaluated, StartTime: time.Now().Add(time.Hour), DiscoveredAt: time.Now()}
	id, _ := d.InsertOpportunity(ctx, o)
	o.ID = id
	d.SaveEvaluationWithPicks(ctx, core.Evaluation{HuntName: o.HuntName, EvaluatedAt: time.Now()}, []core.Pick{{OpportunityID: id, Attributes: []byte("{}")}})
	p, _ := d.GetPicksForOpportunity(ctx, id)
	d.SaveEvaluationWithPicks(ctx, core.Evaluation{HuntName: o.HuntName, EvaluatedAt: time.Now()}, []core.Pick{{OpportunityID: id}})
	ok, e := d.ApplyReviewBackfill(ctx, o, p[0], nil, "audit", .1, "test", time.Now())
	if e != nil || ok {
		t.Fatalf("stale apply %v %v", ok, e)
	}
}

func TestReviewBackfillRejectsChangedIdentity(t *testing.T) {
	d := openTestDB(t)
	ctx := context.Background()
	o := core.Opportunity{HuntName: "performing-arts", SourceID: "x", Title: "Hamilton", Subtitle: "Original cast", State: core.Evaluated, DiscoveredAt: time.Now()}
	id, _ := d.InsertOpportunity(ctx, o)
	o.ID = id
	d.SaveEvaluationWithPicks(ctx, core.Evaluation{HuntName: o.HuntName, EvaluatedAt: time.Now()}, []core.Pick{{OpportunityID: id}})
	p, _ := d.GetPicksForOpportunity(ctx, id)
	d.db.Exec("UPDATE opportunities SET subtitle='New cast' WHERE id=?", id)
	ok, e := d.ApplyReviewBackfill(ctx, o, p[0], nil, "audit", .1, "test", time.Now())
	if e != nil || ok {
		t.Fatalf("accepted changed cast %v %v", ok, e)
	}
}
