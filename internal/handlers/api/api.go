package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	DB *sql.DB
}

func New(db *sql.DB) *Handler { return &Handler{DB: db} }

type healthResponse struct {
	Status string    `json:"status"`
	Time   time.Time `json:"time"`
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Time: time.Now().UTC()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func extractVideoID(url string) string {
	patterns := []string{"v=", "youtu.be/", "embed/", "shorts/"}
	for _, p := range patterns {
		idx := strings.Index(url, p)
		if idx >= 0 {
			start := idx + len(p)
			end := strings.IndexAny(url[start:], "?&#")
			if end < 0 {
				end = len(url[start:])
			}
			id := url[start : start+end]
			if len(id) >= 6 {
				return id
			}
		}
	}
	return ""
}
