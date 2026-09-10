package storage

import (
	"context"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// SavePick inserts a single pick.
func (d *DB) SavePick(ctx context.Context, p core.Pick) (int64, error) {
	attrs := string(p.Attributes)
	if attrs == "" {
		attrs = "{}"
	}
	result, err := d.db.ExecContext(ctx,
		`INSERT INTO picks
		 (evaluation_id, opportunity_id, score, display_score, reason, urgency, attributes)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.EvaluationID, p.OpportunityID, p.Score, p.DisplayScore, p.Reason, p.Urgency, attrs,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetPicksForEvaluation returns all picks for an evaluation.
func (d *DB) GetPicksForEvaluation(ctx context.Context, evaluationID int64) ([]core.Pick, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, evaluation_id, opportunity_id, score, display_score, reason, urgency, attributes
		 FROM picks WHERE evaluation_id = ?
		 ORDER BY score DESC`, evaluationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPicks(rows)
}

// GetPicksForOpportunity returns all picks for an opportunity across evaluations.
func (d *DB) GetPicksForOpportunity(ctx context.Context, opportunityID int64) ([]core.Pick, error) {
	opportunityID, err := d.ResolveOpportunityID(ctx, opportunityID)
	if err != nil {
		return nil, err
	}
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, evaluation_id, opportunity_id, score, display_score, reason, urgency, attributes
		 FROM picks WHERE opportunity_id IN (SELECT id FROM opportunities WHERE id=? OR superseded_by=?)
		 ORDER BY id DESC`, opportunityID, opportunityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPicks(rows)
}

func scanPicks(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]core.Pick, error) {
	var result []core.Pick
	for rows.Next() {
		var p core.Pick
		var attrsStr string
		err := rows.Scan(&p.ID, &p.EvaluationID, &p.OpportunityID,
			&p.Score, &p.DisplayScore, &p.Reason, &p.Urgency, &attrsStr)
		if err != nil {
			return nil, err
		}
		p.Attributes = core.Attributes(attrsStr)
		result = append(result, p)
	}
	return result, rows.Err()
}
