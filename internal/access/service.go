package access

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"math/big"
	"time"
)

var (
	ErrInvalidCode  = errors.New("invalid or expired access code")
	ErrCodeNotFound = errors.New("access code not found")
)

const (
	// CodeLength is the length of generated access codes (8-digit numeric).
	CodeLength = 8
	// DefaultExpiryMonths is the fixed validity period.
	DefaultExpiryMonths = 1
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

// Generate creates a new 8-digit numeric access code valid for 3 months.
// Before generating, it deactivates all existing active codes — only one code
// can be active at a time.
func (s *Service) Generate(ctx context.Context) (*Code, error) {
	// Deactivate all currently active codes first.
	_, err := s.db.ExecContext(ctx,
		`UPDATE access_codes SET is_active = 0 WHERE is_active = 1`,
	)
	if err != nil {
		return nil, err
	}

	code, err := randomNumericCode(CodeLength)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	expires := now.AddDate(0, DefaultExpiryMonths, 0)

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
	if !c.IsActive || time.Now().UTC().After(c.ExpiresAt) {
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
		 WHERE id = ? AND is_active = 0 AND expires_at > ?`,
		id, time.Now().UTC(),
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

// randomNumericCode generates an n-digit cryptographically random numeric code.
func randomNumericCode(n int) (string, error) {
	const charset = "0123456789"
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[idx.Int64()]
	}
	return string(b), nil
}
