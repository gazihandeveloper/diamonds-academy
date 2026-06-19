package frontend

import (
	"errors"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/session"
	"github.com/diamondsacademy/diamonds/internal/views/pages"
)

type AuthHandler struct {
	SM   *scs.SessionManager
	Auth *auth.Service
}

func NewAuth(sm *scs.SessionManager, a *auth.Service) *AuthHandler {
	return &AuthHandler{SM: sm, Auth: a}
}

func (h *AuthHandler) LoginGet(w http.ResponseWriter, r *http.Request) {
	if h.SM.GetInt64(r.Context(), session.KeyUserID) != 0 {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	render(w, r, pages.Login(pages.LoginProps{}))
}

func (h *AuthHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	email := r.FormValue("email")
	password := r.FormValue("password")

	u, err := h.Auth.Authenticate(r.Context(), email, password)
	if err != nil {
		msg := "Geçersiz e-posta veya şifre."
		if !errors.Is(err, auth.ErrInvalidCredentials) {
			msg = "Beklenmeyen bir hata oluştu."
		}
		render(w, r, pages.Login(pages.LoginProps{Email: email, Error: msg}))
		return
	}

	// Only admins can login
	if u.Role != auth.RoleAdmin {
		render(w, r, pages.Login(pages.LoginProps{Email: email, Error: "Bu sayfaya sadece yöneticiler erişebilir."}))
		return
	}

	if err := h.SM.RenewToken(r.Context()); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	h.SM.Put(r.Context(), session.KeyUserID, u.ID)
	h.SM.Put(r.Context(), session.KeyRole, string(u.Role))
	h.SM.Put(r.Context(), session.KeyName, u.Name)
	h.SM.Put(r.Context(), session.KeyEmail, u.Email)
	h.SM.Put(r.Context(), session.KeyMustChangePassword, u.MustChangePassword)

	if u.MustChangePassword {
		http.Redirect(w, r, "/change-password", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	_ = h.SM.Destroy(r.Context())
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) ChangePasswordGet(w http.ResponseWriter, r *http.Request) {
	uid := h.SM.GetInt64(r.Context(), session.KeyUserID)
	if uid == 0 {
		http.Redirect(w, r, "/access", http.StatusSeeOther)
		return
	}
	render(w, r, pages.ChangePassword(pages.ChangePasswordProps{}))
}

func (h *AuthHandler) ChangePasswordPost(w http.ResponseWriter, r *http.Request) {
	uid := h.SM.GetInt64(r.Context(), session.KeyUserID)
	if uid == 0 {
		http.Redirect(w, r, "/access", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	password := r.FormValue("password")
	confirm := r.FormValue("password_confirm")

	if len(password) < 6 {
		render(w, r, pages.ChangePassword(pages.ChangePasswordProps{Error: "Şifre en az 6 karakter olmalı."}))
		return
	}
	if password != confirm {
		render(w, r, pages.ChangePassword(pages.ChangePasswordProps{Error: "Şifreler eşleşmiyor."}))
		return
	}

	if err := h.Auth.SetPassword(r.Context(), uid, password); err != nil {
		render(w, r, pages.ChangePassword(pages.ChangePasswordProps{Error: "Şifre değiştirilemedi."}))
		return
	}

	h.SM.Put(r.Context(), session.KeyMustChangePassword, false)
	render(w, r, pages.ChangePassword(pages.ChangePasswordProps{Success: "Şifreniz başarıyla değiştirildi. Yönlendiriliyorsunuz..."}))
}
