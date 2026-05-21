package server

import (
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/handlers/admin"
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
	adm := admin.New(d.SM, d.DB)

	// Public (auth gerektirmeyen)
	web.Get("/login", authH.LoginGet)
	web.Post("/login", authH.LoginPost)
	web.Post("/logout", authH.Logout)
	web.Get("/register", authH.RegisterGet)
	web.Post("/register", authH.RegisterPost)

	// Protected: kullanıcı girişi zorunlu
	web.Group(func(prot chi.Router) {
		prot.Use(mw.RequireAuth(d.SM))

	prot.Get("/", front.Dashboard)
	prot.Get("/profile", front.Profile)
	prot.Get("/certificate", front.Certificate)
	prot.Get("/learn/{dayNo}", front.Learn)
		prot.Post("/api/progress", front.ProgressBeat)
		prot.Get("/api/progress/{dayNo}", front.ProgressForDay)
		prot.Post("/api/slot-complete", front.MarkSlot)
		prot.Post("/api/quiz-submit", front.QuizSubmit)

		// Admin: admin rolü zorunlu
		prot.Route("/admin", func(a chi.Router) {
			a.Use(mw.RequireAdmin(d.SM))
			a.Get("/", adm.Index)
			a.Get("/days", adm.DaysList)
			a.Get("/days/new", adm.DayNewGet)
			a.Post("/days/new", adm.DayNewPost)
			a.Get("/days/{id}/edit", adm.DayEditGet)
			a.Post("/days/{id}/edit", adm.DayEditPost)
			a.Post("/days/{id}/delete", adm.DayDelete)
			a.Get("/stats", adm.Stats)
		})
	})

	r.Mount("/", web)

	// REST API (token / session bağımsız iskelet)
	apiH := api.New()
	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/health", apiH.Health)
	})

	return r
}
