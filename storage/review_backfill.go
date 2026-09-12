package storage

import (
	"context"
	"fmt"
	"github.com/seanmeyer/opportunity-hunter/core"
	"math"
	"time"
)

func (d *DB) ReviewBackfillDone(ctx context.Context, pickID int64) (bool, error) {
	var n int
	err := d.db.QueryRowContext(ctx, "SELECT count(*) FROM review_backfills WHERE pick_id=?", pickID).Scan(&n)
	return n > 0, err
}

// ApplyReviewBackfill atomically updates metadata and accounts for successful research.
// A stale snapshot, a prior backfill, or a newer judgment is a no-op.
func (d *DB) ApplyReviewBackfill(ctx context.Context, o core.Opportunity, p core.Pick, evidence []core.ReviewEvidence, audit string, cost float64, model string, checkedAt time.Time) (bool, error) {
	if checkedAt.IsZero() || len(evidence) > 3 || cost < 0 || math.IsNaN(cost) || math.IsInf(cost, 0) {
		return false, fmt.Errorf("invalid backfill")
	}
	for _, e := range evidence {
		if !e.Valid() {
			return false, fmt.Errorf("invalid evidence")
		}
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	attrs := core.WithReviewEvidence(p.Attributes, evidence)
	result, err := tx.ExecContext(ctx, `UPDATE picks SET attributes=? WHERE id=? AND json(attributes)=json(?)
 AND NOT EXISTS(SELECT 1 FROM review_backfills WHERE pick_id=picks.id)
 AND EXISTS(SELECT 1 FROM opportunities WHERE id=? AND superseded_by IS NULL AND title=? AND hunt_name=? AND subtitle=? AND source_id=? AND json(attributes)=json(?) AND state IN ('evaluated','notified','reminded'))
 AND id=(SELECT p2.id FROM picks p2 WHERE p2.opportunity_id IN (SELECT id FROM opportunities WHERE id=? OR superseded_by=?) ORDER BY p2.evaluation_id DESC,(p2.opportunity_id=?) DESC,p2.id DESC LIMIT 1)`, string(attrs), p.ID, string(p.Attributes), o.ID, o.Title, o.HuntName, o.Subtitle, o.SourceID, attrsText(o.Attributes), o.ID, o.ID, o.ID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	now := checkedAt.Format(time.RFC3339)
	if _, err = tx.ExecContext(ctx, "INSERT INTO review_backfills(pick_id,checked_at,evidence_count,audit_json) VALUES(?,?,?,?)", p.ID, now, len(evidence), audit); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO eval_costs(hunt_name,evaluated_at,cost_usd,model,success) VALUES(?,?,?,?,1)", o.HuntName, now, cost, model); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func attrsText(a core.Attributes) string {
	if len(a) == 0 {
		return "{}"
	}
	return string(a)
}
