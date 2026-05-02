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
		http.Redirect(w, r, "/", http.StatusSeeOther)
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

	if err := h.SM.RenewToken(r.Context()); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	h.SM.Put(r.Context(), session.KeyUserID, u.ID)
	h.SM.Put(r.Context(), session.KeyRole, string(u.Role))
	h.SM.Put(r.Context(), session.KeyName, u.Name)
	h.SM.Put(r.Context(), session.KeyEmail, u.Email)

	dest := "/"
	if u.Role == auth.RoleAdmin {
		dest = "/admin"
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	_ = h.SM.Destroy(r.Context())
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) RegisterGet(w http.ResponseWriter, r *http.Request) {
	// Zaten giriş yapmışsa anasayfa
	if h.SM.GetInt64(r.Context(), session.KeyUserID) != 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	render(w, r, pages.Register(pages.RegisterProps{}))
}

func (h *AuthHandler) RegisterPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	email := r.FormValue("email")
	name := r.FormValue("name")
	phone := r.FormValue("phone")
	password := r.FormValue("password")
	confirm := r.FormValue("password_confirm")

	fail := func(msg string) {
		render(w, r, pages.Register(pages.RegisterProps{Email: email, Name: name, Phone: phone, Error: msg}))
	}

	if len(password) < 6 {
		fail("Şifre en az 6 karakter olmalı.")
		return
	}
	if password != confirm {
		fail("Şifreler eşleşmiyor.")
		return
	}
	if len(name) < 2 {
		fail("Ad Soyad en az 2 karakter olmalı.")
		return
	}
	if len(phone) < 7 {
		fail("Geçerli bir telefon numarası girin.")
		return
	}

	u, err := h.Auth.Register(r.Context(), email, name, phone, password, auth.RoleUser)
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			fail("Bu e-posta zaten kayıtlı.")
			return
		}
		fail("Kayıt sırasında hata: " + err.Error())
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
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
