package admin

import (
	"net/http"

	"github.com/diamondsacademy/diamonds/internal/progress"
	"github.com/diamondsacademy/diamonds/internal/views/pages"
)

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	repo := progress.NewRepo(h.DB)
	stats, err := repo.UserStats(r.Context())
	if err != nil {
		http.Error(w, "db: "+err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, r, pages.AdminStats(pages.AdminStatsProps{Stats: stats}))
}
