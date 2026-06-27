package frontend

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/oauth"
	"github.com/diamondsacademy/diamonds/internal/session"

	"golang.org/x/oauth2"
)

const (
	oauthStateLen          = 32
	oauthStateCookieName   = "oauth_state"
	oauthStateCookieMaxAge = 600 // 10 minutes
)

// OAuthHandler wraps the session manager, auth service, and per-provider configs.
type OAuthHandler struct {
	SM        *scs.SessionManager
	AuthSvc   *auth.Service
	Google    *oauth2.Config
	Apple     *oauth.AppleProvider
	Instagram *oauth.InstagramProvider
}

// NewOAuthHandler creates a handler for all OAuth providers.
func NewOAuthHandler(sm *scs.SessionManager, authSvc *auth.Service, googleCfg *oauth2.Config, appleCfg *oauth.AppleProvider, instagramCfg *oauth.InstagramProvider) *OAuthHandler {
	return &OAuthHandler{SM: sm, AuthSvc: authSvc, Google: googleCfg, Apple: appleCfg, Instagram: instagramCfg}
}

// ---- Google ----

func (h *OAuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if h.Google == nil {
		http.Error(w, "google auth not configured", http.StatusServiceUnavailable)
		return
	}
	state, err := generateState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	setOAuthStateCookie(w, state)
	http.Redirect(w, r, h.Google.AuthCodeURL(state), http.StatusFound)
}

func (h *OAuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if h.Google == nil {
		http.Error(w, "google auth not configured", http.StatusServiceUnavailable)
		return
	}
	expectedState := getOAuthStateCookie(r)
	clearOAuthStateCookie(w)

	queryState := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if expectedState == "" || queryState != expectedState {
		slog.Warn("google oauth state mismatch", "expected_len", len(expectedState), "got_len", len(queryState))
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	if code == "" {
		slog.Warn("google oauth denied")
		http.Redirect(w, r, "/?error=google_denied", http.StatusSeeOther)
		return
	}

	userInfo, err := oauth.ExchangeGoogleCode(r.Context(), h.Google, code)
	if err != nil {
		slog.Error("google oauth exchange failed", "err", err)
		http.Redirect(w, r, "/?error=google_exchange", http.StatusSeeOther)
		return
	}

	u, err := h.AuthSvc.FindOrCreateByEmail(r.Context(), userInfo.Email, userInfo.Name)
	if err != nil {
		slog.Error("find or create user failed", "err", err)
		http.Redirect(w, r, "/?error=user_creation", http.StatusSeeOther)
		return
	}

	h.loginUser(w, r, u)
}

// ---- Apple ----

func (h *OAuthHandler) AppleLogin(w http.ResponseWriter, r *http.Request) {
	if h.Apple == nil || h.Apple.ServiceID == "" {
		http.Redirect(w, r, "/?error=apple_not_configured", http.StatusSeeOther)
		return
	}
	state, err := generateState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	setOAuthStateCookie(w, state)
	http.Redirect(w, r, h.Apple.AuthCodeURL(state), http.StatusFound)
}

func (h *OAuthHandler) AppleCallback(w http.ResponseWriter, r *http.Request) {
	if h.Apple == nil || h.Apple.ServiceID == "" {
		http.Redirect(w, r, "/?error=apple_not_configured", http.StatusSeeOther)
		return
	}

	expectedState := getOAuthStateCookie(r)
	clearOAuthStateCookie(w)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	queryState := r.FormValue("state")
	code := r.FormValue("code")

	slog.Info("apple callback debug",
		"expected_state_len", len(expectedState),
		"query_state_len", len(queryState),
		"states_match", expectedState == queryState,
		"has_code", code != "",
	)

	if expectedState == "" || queryState != expectedState {
		slog.Warn("apple oauth state mismatch")
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	if code == "" {
		slog.Warn("apple oauth denied")
		http.Redirect(w, r, "/?error=apple_denied", http.StatusSeeOther)
		return
	}

	// Apple sends user info (name) ONLY on first login in the form_post body.
	var appleName string
	if userJSON := r.FormValue("user"); userJSON != "" {
		appleName = oauth.ParseAppleUserName(userJSON)
	}

	userInfo, err := h.Apple.ExchangeAppleCode(code)
	if err != nil {
		slog.Error("apple oauth exchange failed", "err", err)
		http.Redirect(w, r, "/?error=apple_exchange", http.StatusSeeOther)
		return
	}

	email := userInfo.Email
	name := appleName
	if name == "" {
		name = userInfo.Name
	}
	if email == "" {
		email = userInfo.Sub + "@appleid.user"
	}
	if name == "" {
		name = "Apple User"
	}

	u, err := h.AuthSvc.FindOrCreateByEmail(r.Context(), email, name)
	if err != nil {
		slog.Error("find or create user failed", "err", err)
		http.Redirect(w, r, "/?error=user_creation", http.StatusSeeOther)
		return
	}

	h.loginUser(w, r, u)
}

// ---- Instagram / Facebook ----

func (h *OAuthHandler) InstagramLogin(w http.ResponseWriter, r *http.Request) {
	if h.Instagram == nil || h.Instagram.Config == nil || h.Instagram.Config.ClientID == "" {
		http.Redirect(w, r, "/?error=instagram_not_configured", http.StatusSeeOther)
		return
	}
	state, err := generateState()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	setOAuthStateCookie(w, state)
	http.Redirect(w, r, h.Instagram.Config.AuthCodeURL(state), http.StatusFound)
}

func (h *OAuthHandler) InstagramCallback(w http.ResponseWriter, r *http.Request) {
	if h.Instagram == nil || h.Instagram.Config == nil || h.Instagram.Config.ClientID == "" {
		http.Redirect(w, r, "/?error=instagram_not_configured", http.StatusSeeOther)
		return
	}
	expectedState := getOAuthStateCookie(r)
	clearOAuthStateCookie(w)

	queryState := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if expectedState == "" || queryState != expectedState {
		slog.Warn("instagram oauth state mismatch")
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	if code == "" {
		slog.Warn("instagram oauth denied")
		http.Redirect(w, r, "/?error=instagram_denied", http.StatusSeeOther)
		return
	}

	userInfo, err := h.Instagram.ExchangeInstagramCode(r.Context(), code)
	if err != nil {
		slog.Error("instagram oauth exchange failed", "err", err)
		http.Redirect(w, r, "/?error=instagram_exchange", http.StatusSeeOther)
		return
	}

	email := userInfo.Email
	if email == "" {
		email = userInfo.ID + "@facebook.user"
	}
	u, err := h.AuthSvc.FindOrCreateByEmail(r.Context(), email, userInfo.Name)
	if err != nil {
		slog.Error("find or create user failed", "err", err)
		http.Redirect(w, r, "/?error=user_creation", http.StatusSeeOther)
		return
	}

	h.loginUser(w, r, u)
}

// ---- Shared helpers ----

func (h *OAuthHandler) loginUser(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if err := h.SM.RenewToken(r.Context()); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	h.SM.Put(r.Context(), session.KeyUserID, u.ID)
	h.SM.Put(r.Context(), session.KeyRole, string(u.Role))
	h.SM.Put(r.Context(), session.KeyName, u.Name)
	h.SM.Put(r.Context(), session.KeyEmail, u.Email)
	// access_granted is NOT set here — user must enter access code after login

	// If the OAuth provider didn't give us a real name, force the user to enter one.
	if isOAuthPlaceholderName(u.Name) {
		h.SM.Put(r.Context(), session.KeyNameNeeded, true)
		http.Redirect(w, r, "/set-name", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// isOAuthPlaceholderName returns true if the name looks like a system-generated
// placeholder rather than a real person name (e.g. "Ziyaretçi", "Apple User",
// email-as-name, empty).
func isOAuthPlaceholderName(name string) bool {
	if name == "" || name == "Ziyaretçi" || name == "Apple User" {
		return true
	}
	// Email-as-name: contains @ but no spaces
	if strings.Contains(name, "@") && !strings.Contains(name, " ") {
		return true
	}
	return false
}

func generateState() (string, error) {
	b := make([]byte, oauthStateLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// setOAuthStateCookie sets a standalone cookie for the OAuth state.
// Uses SameSite=None + Secure for Apple form_post compatibility.
func setOAuthStateCookie(w http.ResponseWriter, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   oauthStateCookieMaxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})
}

// getOAuthStateCookie reads the OAuth state cookie from the request.
func getOAuthStateCookie(r *http.Request) string {
	c, err := r.Cookie(oauthStateCookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

// clearOAuthStateCookie removes the OAuth state cookie.
func clearOAuthStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})
}
