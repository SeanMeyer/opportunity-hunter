package storage

import (
	"context"
	"time"
)

// MonthlySpendResult holds aggregated monthly cost data.
type MonthlySpendResult struct {
	Total  float64
	ByHunt map[string]float64
}

// RecordCost records an LLM API cost.
func (d *DB) RecordCost(ctx context.Context, huntName string, cost float64, model string, success bool) error {
	successInt := 0
	if success {
		successInt = 1
	}
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO eval_costs (hunt_name, evaluated_at, cost_usd, model, success)
		 VALUES (?, ?, ?, ?, ?)`,
		huntName, time.Now().Format(time.RFC3339), cost, model, successInt,
	)
	return err
}

// MonthlySpend returns total and per-hunt spend for the month containing t.
func (d *DB) MonthlySpend(ctx context.Context, t time.Time) (MonthlySpendResult, error) {
	monthStart := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)

	rows, err := d.db.QueryContext(ctx,
		`SELECT hunt_name, SUM(cost_usd) FROM eval_costs
		 WHERE evaluated_at >= ? AND evaluated_at < ?
		 GROUP BY hunt_name`,
		monthStart.Format(time.RFC3339), monthEnd.Format(time.RFC3339),
	)
	if err != nil {
		return MonthlySpendResult{}, err
	}
	defer rows.Close()

	result := MonthlySpendResult{ByHunt: make(map[string]float64)}
	for rows.Next() {
		var hunt string
		var cost float64
		if err := rows.Scan(&hunt, &cost); err != nil {
			return result, err
		}
		result.ByHunt[hunt] = cost
		result.Total += cost
	}
	return result, rows.Err()
}
