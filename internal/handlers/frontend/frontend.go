package frontend

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"

	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/days"
	"github.com/diamondsacademy/diamonds/internal/progress"
	"github.com/diamondsacademy/diamonds/internal/session"
	"github.com/diamondsacademy/diamonds/internal/quiz"
	"github.com/diamondsacademy/diamonds/internal/views/components"
	"github.com/diamondsacademy/diamonds/internal/views/pages"
)

type Handler struct {
	SM       *scs.SessionManager
	DB       *sql.DB
	Days     *days.Repo
	Progress *progress.Repo
	AuthSvc  *auth.Service
}

func New(sm *scs.SessionManager, db *sql.DB, authSvc *auth.Service) *Handler {
	return &Handler{SM: sm, DB: db, Days: days.NewRepo(db), Progress: progress.NewRepo(db), AuthSvc: authSvc}
}

// sidebarFromSession builds sidebar props from session data.
func (h *Handler) sidebarFromSession(r *http.Request) components.SidebarProps {
	uid := h.SM.GetInt64(r.Context(), session.KeyUserID)
	name := h.SM.GetString(r.Context(), session.KeyName)
	role := h.SM.GetString(r.Context(), session.KeyRole)

	return components.SidebarProps{
		UserName:   name,
		IsLoggedIn: uid != 0,
		IsAdmin:    role == "admin",
	}
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	all, err := h.Days.List(r.Context())
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Sadece yayınlanmış eğitimler dashboard'da görünür
	published := make([]days.Day, 0, len(all))
	for _, d := range all {
		if d.Published {
			published = append(published, d)
		}
	}

	total := len(published)
	if total == 0 {
		render(w, r, pages.DashboardEmpty(h.sidebarFromSession(r), ""))
		return
	}

	// ?day=N ile aktif eğitim seçimi (yoksa ilk tamamlanmamış eğitim)
	uid := h.SM.GetInt64(r.Context(), session.KeyUserID)
	completedQuizzes, _ := h.Progress.CompletedQuizzes(r.Context(), uid)
	completedDays, _ := h.Progress.CompletedDays(r.Context(), uid)

	current := published[0]
	// İlk tamamlanmamış eğitim varsayılan
	for _, d := range published {
		if !completedDays[d.DayNo] {
			current = d
			break
		}
	}
	if q := r.URL.Query().Get("day"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			for _, d := range published {
				if d.DayNo == n {
					current = d
					break
				}
			}
		}
	}

	allCompleted := total > 0
	for _, d := range published {
		if !completedDays[d.DayNo] {
			allCompleted = false
			break
		}
	}

	render(w, r, pages.Dashboard(pages.DashboardProps{
		Sidebar:          h.sidebarFromSession(r),
		Days:             published,
		CurrentDay:       current.DayNo,
		DayTitle:         current.Title,
		Bullets:          current.Bullets,
		Description:      current.Description,
		CompletedDays:    completedDays,
		CompletedQuizzes: completedQuizzes,
		AllCompleted:     allCompleted,
	}))
}

func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = c.Render(r.Context(), w)
}

func (h *Handler) Learn(w http.ResponseWriter, r *http.Request) {
	dayNo, _ := strconv.Atoi(chi.URLParam(r, "dayNo"))
	d, err := h.Days.GetByDayNo(r.Context(), dayNo)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	tab := r.URL.Query().Get("tab")
	switch tab {
	case "l1", "l2", "l3", "file", "quiz":
	default:
		tab = "l1"
	}

	uid := h.SM.GetInt64(r.Context(), session.KeyUserID)

	// Bir sonraki eğitime geçmek için önceki eğitimin quiz'i %100 tamamlanmış olmalı
	// Eğitim 1 ve anon kullanıcı hariç.
	if uid != 0 && dayNo > 1 {
		qDone, _ := h.Progress.QuizCompleted(r.Context(), uid, dayNo-1)
		if !qDone {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
	}

	completed, _ := h.Progress.CompletedSlots(r.Context(), uid, dayNo)

	// Tüm slotlar her zaman açık — sıralı kilit yok.
	questions := quiz.Parse(d.QuizJSON)

	render(w, r, pages.Learn(pages.LearnProps{
		Day:       *d,
		ActiveTab: tab,
		UserEmail: h.SM.GetString(r.Context(), session.KeyEmail),
		Completed: completed,
		Unlocked:  allUnlocked(),
		Questions: questions,
	}))
}

// allUnlocked: tüm slotlar her zaman erişilebilir.
func allUnlocked() map[string]bool {
	return map[string]bool{"l1": true, "l2": true, "l3": true, "file": true, "quiz": true}
}

func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	uid := h.SM.GetInt64(r.Context(), session.KeyUserID)
	u, err := h.AuthSvc.GetByID(r.Context(), uid)
	if err != nil {
		http.Error(w, "kullanıcı bulunamadı", http.StatusInternalServerError)
		return
	}

	dc, _ := h.Progress.CompletedDays(r.Context(), uid)
	daysCompleted := 0
	for _, v := range dc {
		if v {
			daysCompleted++
		}
	}

	var slotsCompleted int
	_ = h.DB.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM slot_completion WHERE user_id = ?`, uid).Scan(&slotsCompleted)

	render(w, r, pages.Profile(pages.ProfileProps{
		Sidebar:        h.sidebarFromSession(r),
		User:           *u,
		DaysCompleted:  daysCompleted,
		SlotsCompleted: slotsCompleted,
	}))
}
