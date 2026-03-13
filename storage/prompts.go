package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// PromptTemplate holds a versioned prompt template.
type PromptTemplate struct {
	HuntName  string
	Version   string
	Template  string
	Active    bool
	CreatedAt time.Time
}

// GetActivePrompt returns the active prompt template for a hunt.
// Returns ErrNotFound if no active template exists.
func (d *DB) GetActivePrompt(ctx context.Context, huntName string) (PromptTemplate, error) {
	var pt PromptTemplate
	var active int
	var createdStr string
	err := d.db.QueryRowContext(ctx,
		`SELECT hunt_name, version, template, active, created_at
		 FROM prompt_templates WHERE hunt_name = ? AND active = 1
		 ORDER BY created_at DESC LIMIT 1`, huntName,
	).Scan(&pt.HuntName, &pt.Version, &pt.Template, &active, &createdStr)
	if errors.Is(err, sql.ErrNoRows) {
		return pt, ErrNotFound
	}
	if err != nil {
		return pt, err
	}
	pt.Active = active != 0
	pt.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	return pt, nil
}

// SavePromptTemplate inserts a new prompt template version.
// If active is true, deactivates all other templates for this hunt.
func (d *DB) SavePromptTemplate(ctx context.Context, pt PromptTemplate) error {
	if pt.Active {
		d.db.ExecContext(ctx,
			`UPDATE prompt_templates SET active = 0 WHERE hunt_name = ?`, pt.HuntName)
	}

	active := 0
	if pt.Active {
		active = 1
	}

	_, err := d.db.ExecContext(ctx,
		`INSERT INTO prompt_templates (hunt_name, version, template, active, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(hunt_name, version) DO UPDATE SET
		   template = excluded.template,
		   active = excluded.active`,
		pt.HuntName, pt.Version, pt.Template, active,
		time.Now().Format(time.RFC3339),
	)
	return err
}
