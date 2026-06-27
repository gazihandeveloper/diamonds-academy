package session

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
)

const (
	KeyUserID             = "user_id"
	KeyRole               = "role"
	KeyName               = "name"
	KeyEmail              = "email"
	KeyAccessGranted      = "access_granted"
	KeyAccessAttempts     = "access_attempts"
	KeyMustChangePassword = "must_change_password"
	KeyTheme              = "theme"
	KeyLocale             = "locale"
	KeyFlash              = "flash"
	KeyNameNeeded         = "name_needed"
)

func New(db *sql.DB, lifetime time.Duration, secure bool) *scs.SessionManager {
	m := scs.New()
	m.Store = sqlite3store.New(db)
	m.Lifetime = lifetime
	m.IdleTimeout = lifetime
	m.Cookie.Name = "diamonds_session"
	m.Cookie.HttpOnly = true
	m.Cookie.Persist = true

	// Apple Sign In uses form_post (cross-site POST).
	// SameSite=Lax blocks cookies on cross-site POST → OAuth state is lost.
	// SameSite=None+Secure is required for Apple callback to receive the session cookie.
	// Google OAuth returns via GET (top-level navigation), which Lax allows —
	// but None is also fine for Google. So we use None in production, Lax in dev.
	if secure {
		m.Cookie.SameSite = http.SameSiteNoneMode
		m.Cookie.Secure = true
	} else {
		m.Cookie.SameSite = http.SameSiteLaxMode
		m.Cookie.Secure = false
	}

	return m
}
