package server

import (
	"database/sql"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/diamondsacademy/diamonds/internal/access"
	"github.com/diamondsacademy/diamonds/internal/auth"
	adm "github.com/diamondsacademy/diamonds/internal/handlers/admin"
	"github.com/diamondsacademy/diamonds/internal/handlers/api"
	"github.com/diamondsacademy/diamonds/internal/handlers/frontend"
	"github.com/diamondsacademy/diamonds/internal/i18n"
	mw "github.com/diamondsacademy/diamonds/internal/middleware"
)

type Deps struct {
	Logger  *slog.Logger
	DB      *sql.DB
	SM      *scs.SessionManager
	AuthSvc *auth.Service
}

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(mw.Logger(d.Logger))
	r.Use(chimw.Recoverer)

	// Statik dosyalar
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static"))))

	// Tüm web rotalarına session sar
	web := chi.NewRouter()
	web.Use(d.SM.LoadAndSave)
	web.Use(i18n.Middleware(d.SM))

	front := frontend.New(d.SM, d.DB, d.AuthSvc)
	authH := frontend.NewAuth(d.SM, d.AuthSvc)
	adminH := adm.New(d.SM, d.DB)
	accessSvc := access.NewService(d.DB)
	accessH := frontend.NewAccessHandler(d.SM, accessSvc)
	adminAccessH := adm.NewAccessHandler(d.SM, accessSvc)
	apiH := api.New(d.DB)

	// Public (auth gerektirmeyen)
	web.Get("/login", authH.LoginGet)
	web.Post("/login", authH.LoginPost)
	web.Post("/logout", authH.Logout)
	web.Get("/register", authH.RegisterGet)
	web.Post("/register", authH.RegisterPost)
	web.Get("/forgot-password", authH.ForgotPasswordGet)
	web.Post("/forgot-password", authH.ForgotPasswordPost)

	// Protected: kullanıcı girişi zorunlu
	web.Group(func(prot chi.Router) {
		prot.Use(mw.RequireAuth(d.SM))

		// Access gate page: authenticated users must pass this before content
		prot.Get("/access", accessH.AccessGet)
		prot.Post("/access", accessH.AccessPost)

		// Access gate: non-admin users must pass access code check
		prot.Group(func(gated chi.Router) {
			gated.Use(mw.RequireAccessGate(d.SM))

			gated.Get("/", front.Dashboard)
			gated.Get("/certificate", front.Certificate)
			gated.Get("/learn/{dayNo}", front.Learn)
			gated.Post("/api/progress", front.ProgressBeat)
			gated.Get("/api/progress/{dayNo}", front.ProgressForDay)
			gated.Post("/api/slot-complete", front.MarkSlot)
			gated.Post("/api/quiz-submit", front.QuizSubmit)
		})

		// Profile, change-password and access page are NOT gated (user needs these without code)
		prot.Get("/profile", front.Profile)
		prot.Get("/change-password", authH.ChangePasswordGet)
		prot.Post("/change-password", authH.ChangePasswordPost)
		prot.Get("/wellbi", front.Wellbi)
		prot.Post("/api/wellbi/chat", apiH.WellbiChat)

		// Admin: admin rolü zorunlu (implicitly bypasses access gate via middleware)
		prot.Route("/admin", func(a chi.Router) {
			a.Use(mw.RequireAdmin(d.SM))
			a.Get("/", adminH.Index)
			a.Get("/days", adminH.DaysList)
			a.Get("/days/new", adminH.DayNewGet)
			a.Post("/days/new", adminH.DayNewPost)
			a.Get("/days/{id}/edit", adminH.DayEditGet)
			a.Post("/days/{id}/edit", adminH.DayEditPost)
			a.Post("/days/{id}/delete", adminH.DayDelete)
			a.Post("/days/auto-translate", adminH.AutoTranslateQuiz)
			a.Post("/days/fetch-transcript", adminH.FetchTranscript)
			// Access code management
			a.Get("/access", adminAccessH.AccessList)
			a.Post("/access/custom", adminAccessH.AccessCustom)
			a.Post("/access/{id}/deactivate", adminAccessH.AccessDeactivate)
			a.Post("/access/{id}/activate", adminAccessH.AccessActivate)
		})
	})

	r.Mount("/", web)

	// REST API (token / session bağımsız iskelet)
	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/health", apiH.Health)
	})
	r.Get("/subtitles", apiH.CaptionsHandler)
	r.Post("/api/captions/save", apiH.SaveCaptionsHandler)
	r.Post("/api/captions/save-transcript", apiH.SaveTranscriptHandler)
	r.Get("/api/captions/fetch-transcript", apiH.FetchTranscriptHandler)

	return r
}

// safeReferer returns the Referer if same-origin, otherwise "/".
func safeReferer(r *http.Request) string {
	ref := r.Header.Get("Referer")
	if ref == "" {
		return "/"
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "/"
	}
	// Only allow same-host redirects
	if !strings.EqualFold(u.Host, r.Host) {
		return "/"
	}
	return ref
}
