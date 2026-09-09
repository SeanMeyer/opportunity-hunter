package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/seanmeyer/opportunity-hunter/core"
	"time"
)

type PendingDelivery struct {
	EvaluationID int64
	GroupKey     string
	Context      core.NotifyContext
	Actions      []core.NotifyAction
	Prepared     bool
}

func (d *DB) PendingDeliveries(ctx context.Context, hunt string) ([]PendingDelivery, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT evaluation_id, group_key, context_json, actions_json FROM pending_deliveries WHERE hunt_name=? AND delivered=0 ORDER BY evaluation_id`, hunt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingDelivery
	for rows.Next() {
		var v PendingDelivery
		var payload string
		var actions sql.NullString
		if err := rows.Scan(&v.EvaluationID, &v.GroupKey, &payload, &actions); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(payload), &v.Context); err != nil {
			return nil, err
		}
		v.Prepared = actions.Valid
		if actions.Valid {
			if err := json.Unmarshal([]byte(actions.String), &v.Actions); err != nil {
				return nil, err
			}
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (d *DB) PrepareDelivery(ctx context.Context, id int64, actions []core.NotifyAction) error {
	payload, err := json.Marshal(actions)
	if err != nil {
		return err
	}
	_, err = d.db.ExecContext(ctx, `UPDATE pending_deliveries SET actions_json=? WHERE evaluation_id=?`, string(payload), id)
	return err
}
func (d *DB) CompleteDelivery(ctx context.Context, id int64) error {
	_, err := d.db.ExecContext(ctx, `UPDATE pending_deliveries SET delivered=1 WHERE evaluation_id=?`, id)
	return err
}

// CheckpointDelivery stores only unacknowledged actions and their resolved thread
// references, atomically with the known created thread ID.
func (d *DB) CheckpointDelivery(ctx context.Context, id int64, hunt, group string, remaining []core.NotifyAction, threads map[string]string) error {
	payload, err := json.Marshal(remaining)
	if err != nil {
		return err
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, threadID := range threads {
		if _, err := tx.ExecContext(ctx, `INSERT INTO notification_threads(hunt_name,group_key,thread_id,created_at) VALUES(?,?,?,?) ON CONFLICT(hunt_name,group_key) DO UPDATE SET thread_id=excluded.thread_id`, hunt, group, threadID, time.Now().Format(time.RFC3339)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pending_deliveries SET actions_json=?,delivered=? WHERE evaluation_id=?`, string(payload), len(remaining) == 0, id); err != nil {
		return err
	}
	return tx.Commit()
}
