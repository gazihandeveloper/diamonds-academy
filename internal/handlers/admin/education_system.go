package admin

import (
	"encoding/json"
	"net/http"

	"github.com/diamondsacademy/diamonds/internal/edusteps"
	"github.com/diamondsacademy/diamonds/internal/views/pages"
)

func (h *Handler) EducationSystem(w http.ResponseWriter, r *http.Request) {
	all, err := h.EduSteps.List(r.Context())
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	steps := make([]pages.EducationStep, 0, len(all))
	for _, s := range all {
		steps = append(steps, pages.EducationStep{
			ID:       s.ID,
			DayNo:    s.StepNo,
			Title:    s.Title,
			Kind:     s.Kind,
			URL:      s.VideoURL,
			QuizJSON: s.QuizJSON,
		})
	}

	render(w, r, pages.AdminEducationSystem(pages.AdminEducationSystemProps{
		Steps: steps,
	}))
}

func (h *Handler) CreateStep(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string `json:"title"`
		Kind     string `json:"kind"`
		URL      string `json:"url"`
		QuizJSON string `json:"quiz_json"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Geçersiz veri"})
		return
	}
	if req.Title == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Başlık zorunlu"})
		return
	}

	nextNo := h.EduSteps.NextStepNo(r.Context())

	step := edusteps.Step{
		StepNo: nextNo,
		Title:  req.Title,
		Kind:   req.Kind,
	}
	if req.Kind == "video" {
		step.VideoURL = req.URL
	} else {
		step.QuizJSON = req.QuizJSON
		if step.QuizJSON == "" {
			step.QuizJSON = "[]"
		}
	}

	if _, err := h.EduSteps.Create(r.Context(), step); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
}

func (h *Handler) DeleteStep(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := h.EduSteps.Delete(r.Context(), req.ID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
}

// EditStep handles POST to update an existing education step.
func (h *Handler) EditStep(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		Kind     string `json:"kind"`
		URL      string `json:"url"`
		QuizJSON string `json:"quiz_json"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Geçersiz veri"})
		return
	}
	if req.Title == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Başlık zorunlu"})
		return
	}
	if err := h.EduSteps.Update(r.Context(), edusteps.Step{
		ID:       req.ID,
		Title:    req.Title,
		Kind:     req.Kind,
		VideoURL: req.URL,
		QuizJSON: req.QuizJSON,
	}); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
}
