package server

import (
	"database/sql"
	"log/slog"
	"net/http"

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
	"github.com/diamondsacademy/diamonds/internal/oauth"
	"golang.org/x/oauth2"
)

type Deps struct {
	Logger           *slog.Logger
	DB               *sql.DB
	SM               *scs.SessionManager
	AuthSvc          *auth.Service
	GoogleOAuth      *oauth2.Config
	AppleProvider    *oauth.AppleProvider
	InstagramProvider *oauth.InstagramProvider
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
	accessH := frontend.NewAccessHandler(d.SM, accessSvc, d.AuthSvc)
	adminAccessH := adm.NewAccessHandler(d.SM, accessSvc)
	apiH := api.New(d.DB, d.SM)

	// OAuth handlers
	oauthH := frontend.NewOAuthHandler(d.SM, d.AuthSvc, d.GoogleOAuth, d.AppleProvider, d.InstagramProvider)
	web.Get("/auth/google", oauthH.GoogleLogin)
	web.Get("/auth/google/callback", oauthH.GoogleCallback)
	web.Get("/auth/apple", oauthH.AppleLogin)
	web.Post("/auth/apple/callback", oauthH.AppleCallback)
	web.Get("/auth/instagram", oauthH.InstagramLogin)
	web.Get("/auth/instagram/callback", oauthH.InstagramCallback)

	// Root: login page (Google/Apple + access code)
	web.Get("/", front.GateLogin)

	// Name entry: OAuth users with placeholder names must enter real name first
	web.Get("/set-name", front.SetNameGet)
	web.Post("/set-name", front.SetNamePost)

	// Legacy access page redirects to root
	web.Get("/access", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	})
	web.Post("/access", accessH.AccessPost)
	web.Post("/logout", authH.Logout)

	// Admin-only login (regular users cannot login)
	web.Get("/login", authH.LoginGet)
	web.Post("/login", authH.LoginPost)

	// Access gate: must pass access code or be OAuth-authenticated
	web.Group(func(gated chi.Router) {
		gated.Use(mw.RequireAccessGate(d.SM))

		gated.Get("/certificate", front.Certificate)
		gated.Get("/profile", front.Profile)
		gated.Post("/profile", front.ProfileUpdate)
		gated.Get("/learn/{stepNo}", func(w http.ResponseWriter, r *http.Request) {
			stepNo := chi.URLParam(r, "stepNo")
			http.Redirect(w, r, "/?step="+stepNo, http.StatusMovedPermanently)
		})
		gated.Post("/api/progress", front.ProgressBeat)
		gated.Get("/api/progress/{dayNo}", front.ProgressForDay)
		gated.Post("/api/slot-complete", front.MarkSlot)
		gated.Post("/api/quiz-submit", front.QuizSubmit)
		gated.Get("/wellbi", front.Wellbi)
		gated.Post("/api/wellbi/chat", apiH.WellbiChat)
	})

	// Admin: requires login + admin role
	web.Group(func(prot chi.Router) {
		prot.Use(mw.RequireAuth(d.SM))
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
			a.Get("/education-system", adminH.EducationSystem)
			a.Post("/education-system/create", adminH.CreateStep)
			a.Post("/education-system/edit", adminH.EditStep)
			a.Post("/education-system/delete", adminH.DeleteStep)
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

