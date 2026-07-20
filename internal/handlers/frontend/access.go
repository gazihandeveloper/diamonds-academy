package frontend

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/diamondsacademy/diamonds/internal/access"
	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/session"
	"github.com/diamondsacademy/diamonds/internal/views/pages"
)

const (
	maxAccessAttempts     = 5
	accessLockoutDuration = 30 * time.Second
)

// AccessHandler handles the access gate page (enter access code).
type AccessHandler struct {
	SM        *scs.SessionManager
	AccessSvc *access.Service
	AuthSvc   *auth.Service
}

// NewAccessHandler creates a new access gate handler.
func NewAccessHandler(sm *scs.SessionManager, as *access.Service, authSvc *auth.Service) *AccessHandler {
	return &AccessHandler{SM: sm, AccessSvc: as, AuthSvc: authSvc}
}

// AccessGet redirects to the login page. Access code entry is now on /.
func (h *AccessHandler) AccessGet(w http.ResponseWriter, r *http.Request) {
	if h.SM.GetBool(r.Context(), session.KeyAccessGranted) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

// AccessPost validates the submitted access code with rate limiting.
func (h *AccessHandler) AccessPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Already granted? Skip.
	if h.SM.GetBool(r.Context(), session.KeyAccessGranted) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Rate limiting: track failed attempts in session
	attempts := h.SM.GetInt(r.Context(), session.KeyAccessAttempts)
	if attempts >= maxAccessAttempts {
		h.SM.Put(r.Context(), session.KeyAccessAttempts, attempts+1)
		render(w, r, pages.GateLogin(pages.GateLoginProps{
			Error: "Çok fazla başarısız deneme. Lütfen 30 saniye bekleyin.",
		}))
		return
	}

	code := r.FormValue("code")
	// Limit code length to prevent abuse
	if len(code) > 64 {
		render(w, r, pages.GateLogin(pages.GateLoginProps{Error: "Geçersiz erişim kodu."}))
		return
	}
	if code == "" {
		render(w, r, pages.GateLogin(pages.GateLoginProps{Error: "Erişim kodu boş bırakılamaz."}))
		return
	}

	_, err := h.AccessSvc.Validate(r.Context(), code)
	if err != nil {
		// Track failed attempt
		h.SM.Put(r.Context(), session.KeyAccessAttempts, attempts+1)
		msg := "Geçersiz erişim kodu."
		if !errors.Is(err, access.ErrInvalidCode) {
			msg = "Beklenmeyen bir hata oluştu."
		}
		render(w, r, pages.GateLogin(pages.GateLoginProps{Error: msg}))
		return
	}

	// Reset attempts on success
	h.SM.Remove(r.Context(), session.KeyAccessAttempts)
	if err := h.SM.RenewToken(r.Context()); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	h.SM.Put(r.Context(), session.KeyAccessGranted, true)

	// Eğer kullanıcı OAuth ile giriş yapmışsa (user_id zaten varsa),
	// yeni anonim kullanıcı oluşturma — mevcut hesabı kullan.
	existingID := h.SM.GetInt64(r.Context(), session.KeyUserID)
	if existingID != 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Create anonymous user for progress tracking
	u, err := h.AuthSvc.CreateAnonymous(r.Context())
	if err != nil {
		slog.Error("create anonymous user failed", "err", err)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.SM.Put(r.Context(), session.KeyUserID, u.ID)
	h.SM.Put(r.Context(), session.KeyRole, string(u.Role))
	h.SM.Put(r.Context(), session.KeyName, u.Name)
	h.SM.Put(r.Context(), session.KeyEmail, u.Email)

	// Force name entry for anonymous users
	h.SM.Put(r.Context(), session.KeyNameNeeded, true)
	http.Redirect(w, r, "/set-name", http.StatusSeeOther)
}
