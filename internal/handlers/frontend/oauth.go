package frontend

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/oauth"
	"github.com/diamondsacademy/diamonds/internal/session"

	"golang.org/x/oauth2"
)

const (
	oauthStateLen        = 32
	oauthStateSessionKey = "oauth_state"
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
	h.SM.Put(r.Context(), oauthStateSessionKey, state)
	http.Redirect(w, r, h.Google.AuthCodeURL(state), http.StatusFound)
}

func (h *OAuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if h.Google == nil {
		http.Error(w, "google auth not configured", http.StatusServiceUnavailable)
		return
	}
	expectedState := h.SM.GetString(r.Context(), oauthStateSessionKey)
	h.SM.Remove(r.Context(), oauthStateSessionKey)

	queryState := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	if expectedState == "" || queryState != expectedState {
		slog.Warn("oauth state mismatch", "expected", expectedState, "got", queryState)
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
	h.SM.Put(r.Context(), oauthStateSessionKey, state)
	http.Redirect(w, r, h.Apple.AuthCodeURL(state), http.StatusFound)
}

func (h *OAuthHandler) AppleCallback(w http.ResponseWriter, r *http.Request) {
	if h.Apple == nil || h.Apple.ServiceID == "" {
		http.Redirect(w, r, "/?error=apple_not_configured", http.StatusSeeOther)
		return
	}
	expectedState := h.SM.GetString(r.Context(), oauthStateSessionKey)
	h.SM.Remove(r.Context(), oauthStateSessionKey)

	// Apple uses form_post response_mode, so code comes from POST body
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	queryState := r.FormValue("state")
	code := r.FormValue("code")

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

	userInfo, err := h.Apple.ExchangeAppleCode(code)
	if err != nil {
		slog.Error("apple oauth exchange failed", "err", err)
		http.Redirect(w, r, "/?error=apple_exchange", http.StatusSeeOther)
		return
	}

	email := userInfo.Email
	name := userInfo.Name
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
	h.SM.Put(r.Context(), oauthStateSessionKey, state)
	http.Redirect(w, r, h.Instagram.Config.AuthCodeURL(state), http.StatusFound)
}

func (h *OAuthHandler) InstagramCallback(w http.ResponseWriter, r *http.Request) {
	if h.Instagram == nil || h.Instagram.Config == nil || h.Instagram.Config.ClientID == "" {
		http.Redirect(w, r, "/?error=instagram_not_configured", http.StatusSeeOther)
		return
	}
	expectedState := h.SM.GetString(r.Context(), oauthStateSessionKey)
	h.SM.Remove(r.Context(), oauthStateSessionKey)

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

// ---- Shared login helper ----

func (h *OAuthHandler) loginUser(w http.ResponseWriter, r *http.Request, u *auth.User) {
	if err := h.SM.RenewToken(r.Context()); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	h.SM.Put(r.Context(), session.KeyUserID, u.ID)
	h.SM.Put(r.Context(), session.KeyRole, string(u.Role))
	h.SM.Put(r.Context(), session.KeyName, u.Name)
	h.SM.Put(r.Context(), session.KeyEmail, u.Email)
	h.SM.Put(r.Context(), session.KeyAccessGranted, true)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func generateState() (string, error) {
	b := make([]byte, oauthStateLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
