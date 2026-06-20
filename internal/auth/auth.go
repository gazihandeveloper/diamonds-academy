package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
)

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID                int64
	Email             string
	Name              string
	Phone             string
	Role              Role
	CreatedAt         time.Time
	MustChangePassword bool
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Register(ctx context.Context, email, name, phone, password string, role Role) (*User, error) {
	email = normalizeEmail(email)
	if email == "" || password == "" || name == "" {
		return nil, errors.New("email, name and password are required")
	}
	if role == "" {
		role = RoleUser
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users(email, name, phone, password_hash, role) VALUES(?,?,?,?,?)`,
		email, name, phone, string(hash), string(role),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrUserExists
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Email: email, Name: name, Phone: phone, Role: role, CreatedAt: time.Now()}, nil
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (*User, error) {
	email = normalizeEmail(email)
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, phone, password_hash, role, created_at, must_change_password FROM users WHERE email = ?`, email)

	var u User
	var hash string
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Phone, &hash, &u.Role, &u.CreatedAt, &u.MustChangePassword); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return &u, nil
}

func (s *Service) ResetPassword(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if email == "" {
		return errors.New("email is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("diamonds1234"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, must_change_password = 1, updated_at = CURRENT_TIMESTAMP WHERE email = ?`,
		string(hash), email,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *Service) SetPassword(ctx context.Context, userID int64, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("password must be at least 6 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, must_change_password = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		string(hash), userID,
	)
	return err
}

// UpdateName updates the user's display name.
func (s *Service) UpdateName(ctx context.Context, userID int64, name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		name, userID,
	)
	return err
}

func (s *Service) MustChangePassword(ctx context.Context, userID int64) (bool, error) {
	var mcp int
	err := s.db.QueryRowContext(ctx, `SELECT must_change_password FROM users WHERE id = ?`, userID).Scan(&mcp)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrUserNotFound
		}
		return false, err
	}
	return mcp == 1, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, phone, role, created_at, must_change_password FROM users WHERE id = ?`, id)
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Phone, &u.Role, &u.CreatedAt, &u.MustChangePassword); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

// EnsureAdmin creates the seed admin if missing. Idempotent.
func (s *Service) EnsureAdmin(ctx context.Context, email, password string) error {
	if email == "" || password == "" {
		return nil
	}
	email = normalizeEmail(email)
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = ?`, email).Scan(&id)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = s.Register(ctx, email, "Administrator", "", password, RoleAdmin)
	if errors.Is(err, ErrUserExists) {
		return nil
	}
	return err
}

// CreateAnonymous creates a user with no login credentials for access-code-only entry.
// Uses a random email so the UNIQUE constraint is never violated.
func (s *Service) CreateAnonymous(ctx context.Context) (*User, error) {
	email := "anon_" + time.Now().Format("20060102150405") + "@guest.local"
	name := "Ziyaretçi"
	// Random password — never used since login is disabled for non-admins
	hash, err := bcrypt.GenerateFromPassword([]byte(email), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users(email, name, phone, password_hash, role) VALUES(?,?,?,?,?)`,
		email, name, "", string(hash), string(RoleUser),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Email: email, Name: name, Role: RoleUser, CreatedAt: time.Now()}, nil
}

// FindOrCreateByEmail looks up a user by email. If the user does not exist, it creates one.
// If the user exists but has a placeholder name ("Ziyaretçi", email-as-name, etc.),
// it updates the name with the one from the OAuth provider. Returns the user on success.
func (s *Service) FindOrCreateByEmail(ctx context.Context, email, name string) (*User, error) {
	email = normalizeEmail(email)
	if email == "" {
		return nil, errors.New("email is required")
	}
	if name == "" {
		name = email
	}

	u, err := s.getByEmail(ctx, email)
	if err == nil {
		// Update name if it looks like a placeholder (anonymous, email-as-name, or empty)
		if isPlaceholderName(u.Name) && name != email {
			_, _ = s.db.ExecContext(ctx,
				`UPDATE users SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
				name, u.ID,
			)
			u.Name = name
		}
		return u, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	// Generate a short random password (never used — login is via OAuth).
	// Must be ≤72 bytes for bcrypt. Use hex-encoded random bytes.
	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		return nil, err
	}
	randomPass := hex.EncodeToString(randBytes) // 32 chars, well under 72 bytes
	hash, err := bcrypt.GenerateFromPassword([]byte(randomPass), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users(email, name, password_hash, role) VALUES(?,?,?,?)`,
		email, name, string(hash), string(RoleUser),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Email: email, Name: name, Role: RoleUser, CreatedAt: time.Now()}, nil
}

// isPlaceholderName returns true if the name looks like a system-generated placeholder.
func isPlaceholderName(name string) bool {
	if name == "" || name == "Ziyaretçi" {
		return true
	}
	if strings.Contains(name, "@") && !strings.Contains(name, " ") {
		return true
	}
	return false
}

func (s *Service) getByEmail(ctx context.Context, email string) (*User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, phone, role, created_at, must_change_password FROM users WHERE email = ?`, email)
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Phone, &u.Role, &u.CreatedAt, &u.MustChangePassword); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }
