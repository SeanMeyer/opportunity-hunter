package storage

import (
	"context"
	"database/sql"
	"time"
)

// FeedbackRow represents a stored feedback entry.
type FeedbackRow struct {
	ID            int64
	OpportunityID *int64
	HuntName      string
	Title         string
	Rating        string // "up" or "down"
	Note          string
	EvalSummary   string // LLM's one-liner at the time of feedback
	EvalScore     string // display score (tier) at the time of feedback
	CreatedAt     time.Time
}

// SaveFeedback inserts a feedback entry. OpportunityID may be nil (manual feedback).
func (d *DB) SaveFeedback(ctx context.Context, f FeedbackRow) (int64, error) {
	if f.OpportunityID != nil {
		id, err := d.ResolveOpportunityID(ctx, *f.OpportunityID)
		if err != nil {
			return 0, err
		}
		f.OpportunityID = &id
	}
	result, err := d.db.ExecContext(ctx,
		`INSERT INTO feedback (opportunity_id, hunt_name, title, rating, note, eval_summary, eval_score, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.OpportunityID, f.HuntName, f.Title, f.Rating, f.Note,
		f.EvalSummary, f.EvalScore,
		f.CreatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetRecentFeedback returns effective feedback for a hunt, newest first.
// The last saved choice per opportunity wins before applying the limit. Earlier
// choices remain stored for history. Manual entries have no shared identity.
func (d *DB) GetRecentFeedback(ctx context.Context, huntName string, limit int) ([]FeedbackRow, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, COALESCE((SELECT superseded_by FROM opportunities WHERE id=f.opportunity_id), opportunity_id), hunt_name, title, rating, note,
		        COALESCE(eval_summary, ''), COALESCE(eval_score, ''), created_at
		 FROM feedback AS f WHERE hunt_name = ?
		 AND (opportunity_id IS NULL OR NOT EXISTS (
		     SELECT 1 FROM feedback AS newer
		     WHERE newer.hunt_name = f.hunt_name
		       AND COALESCE((SELECT superseded_by FROM opportunities WHERE id=newer.opportunity_id), newer.opportunity_id)
		         = COALESCE((SELECT superseded_by FROM opportunities WHERE id=f.opportunity_id), f.opportunity_id) AND newer.id > f.id
		 ))
		 ORDER BY id DESC LIMIT ?`, huntName, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []FeedbackRow
	for rows.Next() {
		var f FeedbackRow
		var oppID sql.NullInt64
		var createdStr string
		if err := rows.Scan(&f.ID, &oppID, &f.HuntName, &f.Title, &f.Rating, &f.Note,
			&f.EvalSummary, &f.EvalScore, &createdStr); err != nil {
			return nil, err
		}
		if oppID.Valid {
			f.OpportunityID = &oppID.Int64
		}
		f.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		result = append(result, f)
	}
	return result, rows.Err()
}
