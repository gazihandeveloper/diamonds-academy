package access

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrInvalidCode  = errors.New("invalid or expired access code")
	ErrCodeNotFound = errors.New("access code not found")
)

const (
	// SentinelExpiry is a far-future date used for all access codes since expiry is disabled.
	// Codes remain valid indefinitely — deactivation is manual via admin panel.
	SentinelExpiry = "9999-12-31T00:00:00Z"
)

// Code represents an access code row.
type Code struct {
	ID        int64
	Code      string
	IsActive  bool
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Service handles access code operations.
type Service struct {
	db *sql.DB
}

// NewService creates a new access code service.
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// Validate checks if a code exists, is active, and not expired.
// Returns the Code on success, or ErrInvalidCode.
func (s *Service) Validate(ctx context.Context, code string) (*Code, error) {
	if code == "" {
		return nil, ErrInvalidCode
	}
	row := s.db.QueryRowContext(ctx,
		`SELECT id, code, is_active, created_at, expires_at
		 FROM access_codes WHERE code = ?`, code,
	)
	var c Code
	var active int
	if err := row.Scan(&c.ID, &c.Code, &active, &c.CreatedAt, &c.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCode
		}
		return nil, err
	}
	c.IsActive = active == 1
	if !c.IsActive {
		return nil, ErrInvalidCode
	}
	return &c, nil
}

// List returns all access codes ordered by newest first.
func (s *Service) List(ctx context.Context) ([]Code, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, code, is_active, created_at, expires_at
		 FROM access_codes ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Code
	for rows.Next() {
		var c Code
		var active int
		if err := rows.Scan(&c.ID, &c.Code, &active, &c.CreatedAt, &c.ExpiresAt); err != nil {
			return nil, err
		}
		c.IsActive = active == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

// Deactivate disables an access code by ID.
func (s *Service) Deactivate(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE access_codes SET is_active = 0 WHERE id = ? AND is_active = 1`, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrCodeNotFound
	}
	return nil
}

// Activate enables an access code by ID (if not expired).
func (s *Service) Activate(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE access_codes SET is_active = 1
		 WHERE id = ? AND is_active = 0`,
		id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrCodeNotFound
	}
	return nil
}

// SetCustomCode allows a user to set their own access code manually.
// Deactivates all other active codes first, then creates this one.
func (s *Service) SetCustomCode(ctx context.Context, code string) (*Code, error) {
	if len(code) < 4 {
		return nil, errors.New("code must be at least 4 characters")
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE access_codes SET is_active = 0 WHERE is_active = 1`,
	)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	expires := time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO access_codes (code, is_active, created_at, expires_at) VALUES (?, 1, ?, ?)`,
		code, now, expires,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &Code{ID: id, Code: code, IsActive: true, CreatedAt: now, ExpiresAt: expires}, nil
}


