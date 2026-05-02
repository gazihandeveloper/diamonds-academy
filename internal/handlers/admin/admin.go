package admin

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/days"
	"github.com/diamondsacademy/diamonds/internal/views/pages"
)

type Handler struct {
	SM   *scs.SessionManager
	DB   *sql.DB
	Days *days.Repo
}

func New(sm *scs.SessionManager, db *sql.DB) *Handler {
	return &Handler{SM: sm, DB: db, Days: days.NewRepo(db)}
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	users, err := listUsers(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, r, pages.Admin(pages.AdminProps{Users: users}))
}

func listUsers(ctx context.Context, db *sql.DB) ([]auth.User, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, email, name, phone, role, created_at FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []auth.User
	for rows.Next() {
		var u auth.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Phone, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = c.Render(r.Context(), w)
}
