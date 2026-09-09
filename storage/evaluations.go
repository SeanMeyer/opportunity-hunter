package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/seanmeyer/opportunity-hunter/core"
)

// SaveEvaluation inserts an evaluation and returns its ID.
func (d *DB) SaveEvaluation(ctx context.Context, eval core.Evaluation) (int64, error) {
	result, err := d.db.ExecContext(ctx,
		`INSERT INTO evaluations
		 (hunt_name, group_key, evaluated_at, skipped_reasoning, raw_llm_response, rendered_prompt, cost_usd, structured_response)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		eval.HuntName, eval.GroupKey, eval.EvaluatedAt.Format(time.RFC3339),
		eval.SkippedReasoning, eval.RawLLMResponse, eval.RenderedPrompt, eval.CostUSD, eval.StructuredResponse,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// SaveEvaluationWithPicks saves an evaluation and its picks in a single transaction.
func (d *DB) SaveEvaluationWithPicks(ctx context.Context, eval core.Evaluation, picks []core.Pick) (int64, error) {
	return d.saveEvaluation(ctx, eval, picks, nil)
}

// SaveEvaluatedGroup commits the judgment, state transitions and delivery intent together.
func (d *DB) SaveEvaluatedGroup(ctx context.Context, eval core.Evaluation, picks []core.Pick, opps []core.Opportunity) (int64, error) {
	return d.saveEvaluation(ctx, eval, picks, opps)
}

func (d *DB) saveEvaluation(ctx context.Context, eval core.Evaluation, picks []core.Pick, opps []core.Opportunity) (int64, error) {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx,
		`INSERT INTO evaluations
		 (hunt_name, group_key, evaluated_at, skipped_reasoning, raw_llm_response, rendered_prompt, cost_usd, structured_response)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		eval.HuntName, eval.GroupKey, eval.EvaluatedAt.Format(time.RFC3339),
		eval.SkippedReasoning, eval.RawLLMResponse, eval.RenderedPrompt, eval.CostUSD, eval.StructuredResponse,
	)
	if err != nil {
		return 0, err
	}
	evalID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, p := range picks {
		attrs := string(p.Attributes)
		if attrs == "" {
			attrs = "{}"
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO picks
			 (evaluation_id, opportunity_id, score, display_score, reason, urgency, attributes)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			evalID, p.OpportunityID, p.Score, p.DisplayScore, p.Reason, p.Urgency, attrs,
		)
		if err != nil {
			return 0, err
		}
	}

	if opps != nil {
		for _, opp := range opps {
			if _, err := tx.ExecContext(ctx, `UPDATE opportunities SET state = ?, evaluated_at = ? WHERE id = ?`, core.Evaluated, time.Now().Format(time.RFC3339), opp.ID); err != nil {
				return 0, err
			}
		}
		eval.ID = evalID
		payload, err := json.Marshal(core.NotifyContext{Evaluations: []core.Evaluation{eval}, Picks: picks, Opportunities: opps})
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO pending_deliveries(evaluation_id,hunt_name,group_key,context_json) VALUES(?,?,?,?)`, evalID, eval.HuntName, eval.GroupKey, string(payload)); err != nil {
			return 0, err
		}
	}
	return evalID, tx.Commit()
}

// GetEvaluation retrieves an evaluation by ID.
func (d *DB) GetEvaluation(ctx context.Context, id int64) (core.Evaluation, error) {
	var eval core.Evaluation
	var evalAtStr string
	err := d.db.QueryRowContext(ctx,
		`SELECT id, hunt_name, group_key, evaluated_at, skipped_reasoning, raw_llm_response, rendered_prompt, cost_usd, structured_response
		 FROM evaluations WHERE id = ?`, id,
	).Scan(&eval.ID, &eval.HuntName, &eval.GroupKey, &evalAtStr,
		&eval.SkippedReasoning, &eval.RawLLMResponse, &eval.RenderedPrompt, &eval.CostUSD, &eval.StructuredResponse)
	if errors.Is(err, sql.ErrNoRows) {
		return eval, ErrNotFound
	}
	if err != nil {
		return eval, err
	}
	eval.EvaluatedAt, _ = time.Parse(time.RFC3339, evalAtStr)
	return eval, nil
}

// GetLatestEvaluation returns the most recent evaluation for a hunt+group_key.
func (d *DB) GetLatestEvaluation(ctx context.Context, huntName, groupKey string) (core.Evaluation, error) {
	var eval core.Evaluation
	var evalAtStr string
	err := d.db.QueryRowContext(ctx,
		`SELECT id, hunt_name, group_key, evaluated_at, skipped_reasoning, raw_llm_response, rendered_prompt, cost_usd, structured_response
		 FROM evaluations WHERE hunt_name = ? AND group_key = ?
		 ORDER BY evaluated_at DESC LIMIT 1`, huntName, groupKey,
	).Scan(&eval.ID, &eval.HuntName, &eval.GroupKey, &evalAtStr,
		&eval.SkippedReasoning, &eval.RawLLMResponse, &eval.RenderedPrompt, &eval.CostUSD, &eval.StructuredResponse)
	if errors.Is(err, sql.ErrNoRows) {
		return eval, ErrNotFound
	}
	if err != nil {
		return eval, err
	}
	eval.EvaluatedAt, _ = time.Parse(time.RFC3339, evalAtStr)
	return eval, nil
}
