package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/session"
)

// newSessionCtx creates a context with a fresh in-memory session and sets
// the given key-value pairs. The returned context is ready to be used in
// an HTTP request for middleware testing.
//
// Uses sm.Load(ctx, "") which creates a zero-value session without store
// interaction — no external dependencies.
func newSessionCtx(sm *scs.SessionManager, pairs map[string]interface{}) context.Context {
	ctx, err := sm.Load(context.Background(), "")
	if err != nil {
		panic("scs.Load with empty token must not fail: " + err.Error())
	}
	for k, v := range pairs {
		sm.Put(ctx, k, v)
	}
	return ctx
}

// testHandler is a simple HTTP handler that records whether it was called.
type testHandler struct {
	called bool
}

func (h *testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	w.WriteHeader(http.StatusOK)
}

// newMiddleware creates a RequireAccessGate middleware wrapper around the
// given handler using the provided session manager.
func newMiddleware(sm *scs.SessionManager, next http.Handler) http.Handler {
	return RequireAccessGate(sm)(next)
}

// makeRequest creates an httptest request with the given context and returns
// the recorder.
func makeRequest(ctx context.Context, target string) (*httptest.ResponseRecorder, *http.Request) {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req = req.WithContext(ctx)
	return httptest.NewRecorder(), req
}

// ─── Admin bypass ───────────────────────────────────────────────────────────

func TestRequireAccessGate_AdminBypass(t *testing.T) {
	sm := scs.New()
	handler := &testHandler{}
	mw := newMiddleware(sm, handler)

	// Admin role — should always pass through regardless of access_granted.
	tcs := []struct {
		name     string
		granted  interface{} // nil = don't set
	}{
		{"admin with grant", true},
		{"admin without grant", false},
		{"admin grant missing", nil},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			handler.called = false
			pairs := map[string]interface{}{
				session.KeyRole: string(auth.RoleAdmin),
			}
			if tc.granted != nil {
				pairs[session.KeyAccessGranted] = tc.granted
			}
			ctx := newSessionCtx(sm, pairs)
			w, r := makeRequest(ctx, "/dashboard")
			mw.ServeHTTP(w, r)

			if !handler.called {
				t.Error("admin should bypass access gate, handler not called")
			}
			if w.Code == http.StatusSeeOther {
				t.Error("admin should not be redirected to /access")
			}
		})
	}
}

// ─── Happy path: access granted ─────────────────────────────────────────────

func TestRequireAccessGate_AccessGranted(t *testing.T) {
	sm := scs.New()
	handler := &testHandler{}
	mw := newMiddleware(sm, handler)

	// Non-admin with access_granted=true should pass through.
	tcs := []struct {
		name string
		role string
	}{
		{"user role granted", string(auth.RoleUser)},
		{"empty role granted", ""},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			handler.called = false
			ctx := newSessionCtx(sm, map[string]interface{}{
				session.KeyRole:          tc.role,
				session.KeyAccessGranted: true,
			})
			w, r := makeRequest(ctx, "/dashboard")
			mw.ServeHTTP(w, r)

			if !handler.called {
				t.Error("handler should have been called when access_granted=true")
			}
			if w.Code == http.StatusSeeOther {
				t.Error("should not redirect when access granted")
			}
		})
	}
}

// ─── Sad path: no access granted ────────────────────────────────────────────

func TestRequireAccessGate_RedirectWhenNotGranted(t *testing.T) {
	sm := scs.New()
	handler := &testHandler{}
	mw := newMiddleware(sm, handler)

	tcs := []struct {
		name  string
		pairs map[string]interface{}
	}{
		{
			name:  "user role no grant",
			pairs: map[string]interface{}{session.KeyRole: string(auth.RoleUser)},
		},
		{
			name:  "user role grant false",
			pairs: map[string]interface{}{session.KeyRole: string(auth.RoleUser), session.KeyAccessGranted: false},
		},
		{
			name:  "empty role no grant",
			pairs: map[string]interface{}{},
		},
		{
			name:  "empty role grant false",
			pairs: map[string]interface{}{session.KeyAccessGranted: false},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			handler.called = false
			ctx := newSessionCtx(sm, tc.pairs)
			w, r := makeRequest(ctx, "/dashboard")
			mw.ServeHTTP(w, r)

			if handler.called {
				t.Error("handler should NOT be called when access not granted")
			}
			if w.Code != http.StatusSeeOther {
				t.Errorf("status = %d, want %d (303 See Other)", w.Code, http.StatusSeeOther)
			}
			loc := w.Header().Get("Location")
			if loc != "/access" {
				t.Errorf("redirect Location = %q, want %q", loc, "/access")
			}
		})
	}
}

// ─── Edge cases ─────────────────────────────────────────────────────────────

func TestRequireAccessGate_NonStandardPaths(t *testing.T) {
	sm := scs.New()
	handler := &testHandler{}
	mw := newMiddleware(sm, handler)

	// Even on the /access page itself, if not granted, redirect loops.
	// The handler (frontend) is expected to handle this by checking
	// access_granted before rendering, but the middleware itself doesn't
	// special-case any path.
	handler.called = false
	ctx := newSessionCtx(sm, map[string]interface{}{
		session.KeyRole: string(auth.RoleUser),
	})
	w, r := makeRequest(ctx, "/access")
	mw.ServeHTTP(w, r)

	if handler.called {
		t.Error("middleware does not skip /access path — handler should not be called")
	}
	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
}

func TestRequireAccessGate_GrantedValueWrongType(t *testing.T) {
	sm := scs.New()
	handler := &testHandler{}
	mw := newMiddleware(sm, handler)

	// If access_granted is stored as a non-bool type (e.g. string "true"),
	// GetBool returns false (zero value) — user is redirected.
	handler.called = false
	ctx := newSessionCtx(sm, map[string]interface{}{
		session.KeyRole:          string(auth.RoleUser),
		session.KeyAccessGranted: "true", // string, not bool
	})
	w, r := makeRequest(ctx, "/dashboard")
	mw.ServeHTTP(w, r)

	if handler.called {
		t.Error("handler should NOT be called — string 'true' != bool true")
	}
	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
}

func TestRequireAccessGate_RoleValueWrongType(t *testing.T) {
	sm := scs.New()
	handler := &testHandler{}
	mw := newMiddleware(sm, handler)

	// If role is stored as a non-string type, GetString returns "".
	// Since "" != "admin", the access_granted check applies.
	handler.called = false
	ctx := newSessionCtx(sm, map[string]interface{}{
		session.KeyRole:          12345, // int, not string
		session.KeyAccessGranted: true,
	})
	w, r := makeRequest(ctx, "/dashboard")
	mw.ServeHTTP(w, r)

	if !handler.called {
		t.Error("handler should be called — even though role type is wrong, access_granted=true passes")
	}
}

// ─── Rate limiting note ─────────────────────────────────────────────────────
//
// Rate limiting is not implemented in the access gate middleware itself.
// If rate limiting is added (e.g. to the frontend AccessPost handler),
// tests should cover:
//   - Too many attempts from a single IP within a time window
//   - Rate limit counter reset after window expires
//   - Rate limit scoped by IP (behind proxy: X-Forwarded-For)
//   - Separate limits for valid vs. invalid code submissions

// ─── RequireAuth and RequireAdmin smoke tests ───────────────────────────────

func TestRequireAuth_RedirectsWhenNoUser(t *testing.T) {
	sm := scs.New()
	handler := &testHandler{}
	mw := RequireAuth(sm)(handler)

	// No user_id in session → redirect to /login.
	ctx := newSessionCtx(sm, nil)
	w, r := makeRequest(ctx, "/dashboard")
	mw.ServeHTTP(w, r)

	if handler.called {
		t.Error("handler should not be called when no user_id")
	}
	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	if loc := w.Header().Get("Location"); loc != "/access" {
		t.Errorf("Location = %q, want %q", loc, "/access")
	}
}

func TestRequireAuth_PassesWhenUserPresent(t *testing.T) {
	sm := scs.New()
	handler := &testHandler{}
	mw := RequireAuth(sm)(handler)

	ctx := newSessionCtx(sm, map[string]interface{}{
		session.KeyUserID: int64(42),
	})
	w, r := makeRequest(ctx, "/dashboard")
	mw.ServeHTTP(w, r)

	if !handler.called {
		t.Error("handler should be called when user_id is set")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRequireAuth_UserIDZero(t *testing.T) {
	sm := scs.New()
	handler := &testHandler{}
	mw := RequireAuth(sm)(handler)

	// user_id = 0 → same as missing → redirect.
	ctx := newSessionCtx(sm, map[string]interface{}{
		session.KeyUserID: int64(0),
	})
	w, r := makeRequest(ctx, "/dashboard")
	mw.ServeHTTP(w, r)

	if handler.called {
		t.Error("handler should not be called when user_id == 0")
	}
	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
}

func TestRequireAdmin_ForbidsNonAdmin(t *testing.T) {
	sm := scs.New()
	handler := &testHandler{}
	mw := RequireAdmin(sm)(handler)

	tcs := []struct {
		name string
		role string
	}{
		{"user role", string(auth.RoleUser)},
		{"empty role", ""},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			handler.called = false
			ctx := newSessionCtx(sm, map[string]interface{}{
				session.KeyRole: tc.role,
			})
			w, r := makeRequest(ctx, "/admin")
			mw.ServeHTTP(w, r)

			if handler.called {
				t.Error("handler should not be called for non-admin")
			}
			if w.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
			}
		})
	}
}

func TestRequireAdmin_AllowsAdmin(t *testing.T) {
	sm := scs.New()
	handler := &testHandler{}
	mw := RequireAdmin(sm)(handler)

	ctx := newSessionCtx(sm, map[string]interface{}{
		session.KeyRole: string(auth.RoleAdmin),
	})
	w, r := makeRequest(ctx, "/admin")
	mw.ServeHTTP(w, r)

	if !handler.called {
		t.Error("handler should be called for admin role")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
