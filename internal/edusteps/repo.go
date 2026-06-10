package edusteps

import (
	"context"
	"database/sql"
	"time"
)

// Step represents a single education step (video or quiz).
type Step struct {
	ID       int64
	StepNo   int
	Title    string
	Kind     string // "video" or "quiz"
	VideoURL string
	QuizJSON string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Repo manages education_steps table operations.
type Repo struct{ db *sql.DB }

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

// List returns all steps ordered by step_no.
func (r *Repo) List(ctx context.Context) ([]Step, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, step_no, title, kind, video_url, quiz_json, created_at, updated_at
		 FROM education_steps ORDER BY step_no ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Step
	for rows.Next() {
		var s Step
		if err := rows.Scan(&s.ID, &s.StepNo, &s.Title, &s.Kind, &s.VideoURL, &s.QuizJSON, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// NextStepNo returns the next available step_no (max existing + 1, or 1).
func (r *Repo) NextStepNo(ctx context.Context) int {
	row := r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(step_no), 0) + 1 FROM education_steps`)
	var n int
	_ = row.Scan(&n)
	return n
}

// Create inserts a new education step.
func (r *Repo) Create(ctx context.Context, s Step) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO education_steps (step_no, title, kind, video_url, quiz_json)
		 VALUES (?, ?, ?, ?, ?)`,
		s.StepNo, s.Title, s.Kind, s.VideoURL, s.QuizJSON)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Delete removes a step by ID.
func (r *Repo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM education_steps WHERE id = ?`, id)
	return err
}

// Update modifies an existing step.
func (r *Repo) Update(ctx context.Context, s Step) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE education_steps SET title=?, kind=?, video_url=?, quiz_json=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		s.Title, s.Kind, s.VideoURL, s.QuizJSON, s.ID)
	return err
}

// Count returns total number of steps.
func (r *Repo) Count(ctx context.Context) int {
	row := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM education_steps`)
	var n int
	_ = row.Scan(&n)
	return n
}
