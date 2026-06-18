package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/session"
)

// Logger logs each request with method, path, status, duration.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(rw, r)
			log.Info("http",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rw.status),
				slog.Duration("dur", time.Since(start)),
				slog.String("ip", r.RemoteAddr),
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// RequireAuth redirects to /access if no session user.
func RequireAuth(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := sm.GetInt64(r.Context(), session.KeyUserID)
			if id == 0 {
				http.Redirect(w, r, "/access", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAdmin gates admin routes.
func RequireAdmin(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := sm.GetString(r.Context(), session.KeyRole)
			if role != string(auth.RoleAdmin) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAccessGate redirects to /access if the user hasn't passed the access gate.
// Skips admin routes and the access page itself.
func RequireAccessGate(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Admin users bypass the access gate
			role := sm.GetString(r.Context(), session.KeyRole)
			if role == string(auth.RoleAdmin) {
				next.ServeHTTP(w, r)
				return
			}
			// Check if access has been granted
			granted := sm.GetBool(r.Context(), session.KeyAccessGranted)
			if !granted {
				http.Redirect(w, r, "/access", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
