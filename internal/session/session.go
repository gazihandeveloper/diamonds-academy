package session

import (
	"database/sql"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
)

const (
	KeyUserID        = "user_id"
	KeyRole          = "role"
	KeyName          = "name"
	KeyEmail         = "email"
	KeyAccessGranted = "access_granted"
	KeyTheme         = "theme"
	KeyLocale        = "locale"
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
