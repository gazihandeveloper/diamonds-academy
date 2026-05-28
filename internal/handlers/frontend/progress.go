package frontend

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/diamondsacademy/diamonds/internal/i18n"
	"github.com/diamondsacademy/diamonds/internal/progress"
	"github.com/diamondsacademy/diamonds/internal/quiz"
	"github.com/diamondsacademy/diamonds/internal/session"
)

type heartbeatReq struct {
	DayNo        int     `json:"day_no"`
	Slot         string  `json:"slot"`
	Position     float64 `json:"position"`
	Duration     float64 `json:"duration"`
	Percent      float64 `json:"percent"`
	SecondsDelta float64 `json:"seconds_delta"`
}

func (h *Handler) progressUserID(r *http.Request) int64 {
	return h.SM.GetInt64(r.Context(), session.KeyUserID)
}

func (h *Handler) ProgressBeat(w http.ResponseWriter, r *http.Request) {
	uid := h.progressUserID(r)
	if uid == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req heartbeatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	switch req.Slot {
	case "l1", "l2", "l3":
	default:
		http.Error(w, "bad slot", http.StatusBadRequest)
		return
	}
	if req.SecondsDelta < 0 {
		req.SecondsDelta = 0
	}
	if req.SecondsDelta > 30 {
		req.SecondsDelta = 30
	}
	if err := h.Progress.Upsert(r.Context(), progress.Entry{
		UserID:   uid,
		DayNo:    req.DayNo,
		Slot:     req.Slot,
		Position: req.Position,
		Duration: req.Duration,
		Percent:  req.Percent,
	}, req.SecondsDelta); err != nil {
		http.Error(w, "db: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ProgressForDay(w http.ResponseWriter, r *http.Request) {
	uid := h.progressUserID(r)
	dayNo, _ := strconv.Atoi(chi.URLParam(r, "dayNo"))
	out := map[string]any{}
	if uid == 0 {
		writeJSON(w, out)
		return
	}
	list, err := h.Progress.ListForUserDay(r.Context(), uid, dayNo)
	if err != nil {
		http.Error(w, "db: "+err.Error(), http.StatusInternalServerError)
		return
	}
	for _, e := range list {
		out[e.Slot] = map[string]any{
			"position":  e.Position,
			"duration":  e.Duration,
			"percent":   e.Percent,
			"completed": e.Completed,
		}
	}
	writeJSON(w, out)
}

// MarkSlot: kullanıcı file/quiz slotunu elle tamamladığında.
type markReq struct {
	DayNo int    `json:"day_no"`
	Slot  string `json:"slot"`
}

func (h *Handler) MarkSlot(w http.ResponseWriter, r *http.Request) {
	uid := h.progressUserID(r)
	if uid == 0 {
		writeJSON(w, map[string]any{"ok": false, "reason": "auth"})
		return
	}
	var req markReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	switch req.Slot {
	case "file", "quiz", "l1", "l2", "l3":
	default:
		http.Error(w, "bad slot", http.StatusBadRequest)
		return
	}
	if err := h.Progress.MarkComplete(r.Context(), uid, req.DayNo, req.Slot); err != nil {
		http.Error(w, "db: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// QuizSubmit: kullanıcının cevaplarını değerlendirir. Tümü doğruysa quiz slotu tamamlandı sayılır.
type quizSubmitReq struct {
	DayNo   int   `json:"day_no"`
	Answers []int `json:"answers"`
}

func (h *Handler) QuizSubmit(w http.ResponseWriter, r *http.Request) {
	uid := h.progressUserID(r)
	var req quizSubmitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	d, err := h.Days.GetByDayNo(r.Context(), req.DayNo)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	locale := i18n.FromContext(r.Context())
	qs := quiz.ParseForLocale(d.QuizJSON, d.QuizJSON_EN, d.QuizJSON_BG, locale)
	correct, total := quiz.Grade(qs, req.Answers)

	// Her sorunun doğru index'i kullanıcıya da geri dönülür (öğrenme amaçlı).
	correctIdx := make([]int, len(qs))
	for i, q := range qs {
		correctIdx[i] = q.Correct
	}

	passed := total > 0 && correct == total
	if passed && uid != 0 {
		_ = h.Progress.MarkComplete(r.Context(), uid, req.DayNo, "quiz")
	}

	writeJSON(w, map[string]any{
		"correct":  correct,
		"total":    total,
		"passed":   passed,
		"answers":  correctIdx,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
