package auth

import (
	"context"
	"database/sql"
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
	ID        int64
	Email     string
	Name      string
	Phone     string
	Role      Role
	CreatedAt time.Time
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
		`SELECT id, email, name, phone, password_hash, role, created_at FROM users WHERE email = ?`, email)

	var u User
	var hash string
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Phone, &hash, &u.Role, &u.CreatedAt); err != nil {
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

func (s *Service) GetByID(ctx context.Context, id int64) (*User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, phone, role, created_at FROM users WHERE id = ?`, id)
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Phone, &u.Role, &u.CreatedAt); err != nil {
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

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }
