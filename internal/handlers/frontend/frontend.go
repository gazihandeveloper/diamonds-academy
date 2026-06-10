package frontend

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"

	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/days"
	"github.com/diamondsacademy/diamonds/internal/edusteps"
	"github.com/diamondsacademy/diamonds/internal/i18n"
	"github.com/diamondsacademy/diamonds/internal/progress"
	"github.com/diamondsacademy/diamonds/internal/quiz"
	"github.com/diamondsacademy/diamonds/internal/session"
	"github.com/diamondsacademy/diamonds/internal/steps"
	"github.com/diamondsacademy/diamonds/internal/views/components"
	"github.com/diamondsacademy/diamonds/internal/views/pages"
)

type Handler struct {
	SM       *scs.SessionManager
	DB       *sql.DB
	Days     *days.Repo
	EduSteps *edusteps.Repo
	Progress *progress.Repo
	AuthSvc  *auth.Service
}

func New(sm *scs.SessionManager, db *sql.DB, authSvc *auth.Service) *Handler {
	return &Handler{SM: sm, DB: db, Days: days.NewRepo(db), EduSteps: edusteps.NewRepo(db), Progress: progress.NewRepo(db), AuthSvc: authSvc}
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
	eduList, err := h.EduSteps.List(r.Context())
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(eduList) == 0 {
		render(w, r, pages.DashboardEmpty(h.sidebarFromSession(r), ""))
		return
	}

	// education_steps → steps.Step dönüşümü
	stepList := make([]steps.Step, 0, len(eduList))
	for _, es := range eduList {
		slot := "l1"
		videoURL := es.VideoURL
		if es.Kind == "quiz" {
			slot = "quiz"
			videoURL = ""
		}
		stepList = append(stepList, steps.Step{
			Number:   es.StepNo,
			DayNo:    es.StepNo, // progress için step_no = day_no
			Slot:     slot,
			Kind:     es.Kind,
			VideoURL: videoURL,
			Title:    es.Title,
		})
	}

	// Tamamlanma durumu (step_no'yu day_no olarak kullan)
	uid := h.SM.GetInt64(r.Context(), session.KeyUserID)
	allSlots, _ := h.Progress.AllCompletedSlots(r.Context(), uid)

	stepCompleted := make([]bool, len(stepList))
	firstUnfinished := 0
	for i, s := range stepList {
		if m, ok := allSlots[s.DayNo]; ok && m[s.Slot] {
			stepCompleted[i] = true
		} else if firstUnfinished == 0 {
			firstUnfinished = s.Number
		}
	}

	stepUnlocked := make([]bool, len(stepList))
	for i := range stepList {
		if i == 0 {
			stepUnlocked[i] = true
		} else {
			stepUnlocked[i] = stepCompleted[i-1]
		}
	}

	currentStep := stepList[0].Number
	if firstUnfinished > 0 {
		currentStep = firstUnfinished
	}
	if q := r.URL.Query().Get("step"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n >= 1 && n <= len(stepList) {
			// Sadece unlock edilmiş veya mevcut adıma izin ver
			idx := n - 1
			if idx >= 0 && idx < len(stepUnlocked) && stepUnlocked[idx] {
				currentStep = n
			}
		}
	}

	allCompleted := true
	for _, c := range stepCompleted {
		if !c {
			allCompleted = false
			break
		}
	}

	// Şu anki adımı bul
	currStep := steps.FindByNumber(stepList, currentStep)
	stepTitle := ""
	isQuiz := false
	videoURL := ""
	slot := ""
	dayNo := 0
	var questions []quiz.Question

	if currStep != nil {
		stepTitle = currStep.Title
		isQuiz = currStep.Kind == "quiz"
		videoURL = currStep.VideoURL
		slot = currStep.Slot
		dayNo = currStep.DayNo

		if isQuiz {
			// Education steps'ten quiz JSON'ını bul
			for _, es := range eduList {
				if es.StepNo == currentStep {
					locale := i18n.FromContext(r.Context())
					questions = quiz.ParseForLocale(es.QuizJSON, "", "", locale)
					break
				}
			}
		}
	}

	render(w, r, pages.Dashboard(pages.DashboardProps{
		Sidebar:        h.sidebarFromSession(r),
		Steps:          stepList,
		CurrentStep:    currentStep,
		StepTitle:      stepTitle,
		Bullets:        []string{},
		Description:    "",
		StepCompleted:  stepCompleted,
		StepUnlocked:   stepUnlocked,
		AllCompleted:   allCompleted,
		IsQuiz:         isQuiz,
		VideoURL:       videoURL,
		Slot:           slot,
		DayNo:          dayNo,
		Questions:      questions,
		UserEmail:      h.SM.GetString(r.Context(), session.KeyEmail),
	}))
}

func localizedDayTitle(d days.Day, r *http.Request) string {
	locale := i18n.FromContext(r.Context())
	switch locale {
	case i18n.LocaleEN:
		if d.Title_EN != "" {
			return d.Title_EN
		}
	case i18n.LocaleBG:
		if d.Title_BG != "" {
			return d.Title_BG
		}
	}
	return d.Title
}

func localizedBullets(d days.Day, r *http.Request) []string {
	locale := i18n.FromContext(r.Context())
	switch locale {
	case i18n.LocaleEN:
		if len(d.Bullets_EN) > 0 {
			return d.Bullets_EN
		}
	case i18n.LocaleBG:
		if len(d.Bullets_BG) > 0 {
			return d.Bullets_BG
		}
	}
	return d.Bullets
}

func localizedDescription(d days.Day, r *http.Request) string {
	locale := i18n.FromContext(r.Context())
	switch locale {
	case i18n.LocaleEN:
		if d.Description_EN != "" {
			return d.Description_EN
		}
	case i18n.LocaleBG:
		if d.Description_BG != "" {
			return d.Description_BG
		}
	}
	return d.Description
}

func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = c.Render(r.Context(), w)
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

func (h *Handler) Wellbi(w http.ResponseWriter, r *http.Request) {
	render(w, r, pages.Wellbi(pages.WellbiProps{}))
}
