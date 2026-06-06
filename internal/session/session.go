package session

import (
	"database/sql"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
)

const (
	KeyUserID         = "user_id"
	KeyRole           = "role"
	KeyName           = "name"
	KeyEmail          = "email"
	KeyAccessGranted  = "access_granted"
	KeyAccessAttempts = "access_attempts"
	KeyMustChangePassword = "must_change_password"
	KeyTheme              = "theme"
	KeyLocale         = "locale"
	KeyFlash          = "flash"
)

func New(db *sql.DB, lifetime time.Duration, secure bool) *scs.SessionManager {
	m := scs.New()
	m.Store = sqlite3store.New(db)
	m.Lifetime = lifetime
	m.IdleTimeout = lifetime
	m.Cookie.Name = "diamonds_session"
	m.Cookie.HttpOnly = true
	m.Cookie.Persist = true
	m.Cookie.SameSite = 2 // http.SameSiteLaxMode
	m.Cookie.Secure = secure
	return m
}
