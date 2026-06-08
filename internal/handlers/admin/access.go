package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/diamondsacademy/diamonds/internal/access"
	"github.com/diamondsacademy/diamonds/internal/session"
	"github.com/diamondsacademy/diamonds/internal/views/pages"

	"github.com/alexedwards/scs/v2"
)

// AccessHandler manages access codes from the admin panel.
type AccessHandler struct {
	SM        *scs.SessionManager
	AccessSvc *access.Service
}

// NewAccessHandler creates a new admin access handler.
func NewAccessHandler(sm *scs.SessionManager, as *access.Service) *AccessHandler {
	return &AccessHandler{SM: sm, AccessSvc: as}
}

// AccessList lists all access codes.
func (h *AccessHandler) AccessList(w http.ResponseWriter, r *http.Request) {
	codes, err := h.AccessSvc.List(r.Context())
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	flash := h.SM.PopString(r.Context(), session.KeyFlash)
	render(w, r, pages.AdminAccess(pages.AdminAccessProps{Codes: codes, Flash: flash}))
}

// AccessCustom creates an access code with a user-specified code.
func (h *AccessHandler) AccessCustom(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.setFlash(r, "Hata: geçersiz form")
		http.Redirect(w, r, "/admin/access", http.StatusSeeOther)
		return
	}
	code := strings.TrimSpace(r.FormValue("code"))
	if len(code) < 4 {
		h.setFlash(r, "Kod en az 4 karakter olmalıdır.")
		http.Redirect(w, r, "/admin/access", http.StatusSeeOther)
		return
	}
	_, err := h.AccessSvc.SetCustomCode(r.Context(), code)
	if err != nil {
		h.setFlash(r, "Hata: "+err.Error())
	} else {
		h.setFlash(r, "Erişim kodu oluşturuldu: "+code)
	}
	http.Redirect(w, r, "/admin/access", http.StatusSeeOther)
}

// AccessDeactivate deactivates an access code.
func (h *AccessHandler) AccessDeactivate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.AccessSvc.Deactivate(r.Context(), id); err != nil {
		h.setFlash(r, "Kod bulunamadı veya zaten pasif.")
	} else {
		h.setFlash(r, "Kod deaktif edildi.")
	}
	http.Redirect(w, r, "/admin/access", http.StatusSeeOther)
}

// AccessActivate activates an access code.
func (h *AccessHandler) AccessActivate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.AccessSvc.Activate(r.Context(), id); err != nil {
		h.setFlash(r, "Kod aktif edilemedi (süresi dolmuş olabilir).")
	} else {
		h.setFlash(r, "Kod aktif edildi.")
	}
	http.Redirect(w, r, "/admin/access", http.StatusSeeOther)
}

// setFlash stores a flash message in the session.
func (h *AccessHandler) setFlash(r *http.Request, msg string) {
	h.SM.Put(r.Context(), session.KeyFlash, msg)
}
