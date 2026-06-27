package frontend

import (
	"net/http"
	"strings"

	"github.com/diamondsacademy/diamonds/internal/session"
)

const setNameHTML = `<!DOCTYPE html>
<html lang="tr">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Adınızı Girin — Diamonds Academy</title>
<script src="https://cdn.tailwindcss.com"></script>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
<style>
  body { font-family: "Inter", sans-serif; }
</style>
</head>
<body class="bg-gray-950 text-gray-100 min-h-screen flex items-center justify-center p-4">
  <div class="w-full max-w-md">
    <div class="text-center mb-8">
      <div class="inline-flex items-center justify-center w-16 h-16 bg-gradient-to-br from-purple-600 to-blue-600 rounded-2xl mb-4 shadow-lg shadow-purple-500/20">
        <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 11c0 3.517-1.009 6.799-2.753 9.571m-3.44-2.04l.054-.09A13.916 13.916 0 008 11a4 4 0 118 0c0 1.017-.07 2.019-.203 3m-2.118 6.844A21.88 21.88 0 0015.171 17m3.839 1.132c.645-2.266.99-4.659.99-7.132A8 8 0 008 4.07M3 15.364c.64-1.319 1-2.8 1-4.364 0-1.457.39-2.823 1.07-4"/></svg>
      </div>
      <h1 class="text-2xl font-bold text-white">Hoş Geldiniz!</h1>
      <p class="text-gray-400 text-sm mt-2">Devam etmek için adınızı ve soyadınızı girin.</p>
    </div>
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-8 shadow-2xl">
      {{if .Error}}
      <div class="bg-red-900/40 border border-red-700 text-red-300 rounded-xl px-4 py-3 mb-5 text-sm flex items-center gap-2">
        <svg class="w-5 h-5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4.5c-.77-.833-2.694-.833-3.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z"/></svg>
        {{.Error}}
      </div>
      {{end}}
      <form method="POST" class="space-y-4">
        <div>
          <label class="text-gray-400 text-xs uppercase tracking-wider block mb-1.5">Adınız Soyadınız</label>
          <input type="text" name="name" required autofocus autocomplete="name" placeholder="Ad Soyad"
            value="{{.NameHint}}"
            maxlength="120"
            class="w-full bg-gray-800 border border-gray-700 rounded-xl px-4 py-3.5 text-base text-white outline-none focus:border-purple-500 focus:ring-2 focus:ring-purple-500/20 transition-all placeholder:text-gray-600">
        </div>
        <button type="submit"
          class="w-full bg-gradient-to-r from-purple-600 to-blue-600 hover:from-purple-500 hover:to-blue-500 text-white font-semibold rounded-xl py-3.5 text-base transition-all flex items-center justify-center gap-2 shadow-lg shadow-purple-500/20">
          Devam Et
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7l5 5m0 0l-5 5m5-5H6"/></svg>
        </button>
      </form>
    </div>
  </div>
</body>
</html>`

// SetNameGet shows the name entry form. Only accessible when user is authenticated
// via OAuth but hasn't set their name yet.
func (h *Handler) SetNameGet(w http.ResponseWriter, r *http.Request) {
	uid := h.SM.GetInt64(r.Context(), session.KeyUserID)
	if uid == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	// Already granted AND name is not needed (real name already set) → skip to dashboard
	if h.SM.GetBool(r.Context(), session.KeyAccessGranted) {
		name := h.SM.GetString(r.Context(), session.KeyName)
		if !isOAuthPlaceholderName(name) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}
	// Pre-fill with current name if it's not a placeholder
	nameHint := h.SM.GetString(r.Context(), session.KeyName)
	if nameHint == "Ziyaretçi" || nameHint == "Apple User" || strings.Contains(nameHint, "@") {
		nameHint = ""
	}

	html := strings.Replace(setNameHTML, "{{.Error}}", "", 1)
	html = strings.Replace(html, "{{.NameHint}}", nameHint, 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// SetNamePost processes the name form submission.
func (h *Handler) SetNamePost(w http.ResponseWriter, r *http.Request) {
	uid := h.SM.GetInt64(r.Context(), session.KeyUserID)
	if uid == 0 {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		h.renderSetNameError(w, r, "Ad soyad boş bırakılamaz.")
		return
	}
	if len(name) > 120 {
		h.renderSetNameError(w, r, "Ad soyad çok uzun (en fazla 120 karakter).")
		return
	}

	if err := h.AuthSvc.UpdateName(r.Context(), uid, name); err != nil {
		h.renderSetNameError(w, r, "İsim kaydedilirken hata oluştu. Lütfen tekrar deneyin.")
		return
	}

	// Update session and clear name_needed flag
	h.SM.Put(r.Context(), session.KeyName, name)
	h.SM.Remove(r.Context(), session.KeyNameNeeded)

	// Redirect to access code page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) renderSetNameError(w http.ResponseWriter, r *http.Request, errMsg string) {
	nameHint := r.FormValue("name")
	html := strings.Replace(setNameHTML, "{{.Error}}", errMsg, 1)
	html = strings.Replace(html, "{{.NameHint}}", nameHint, 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
