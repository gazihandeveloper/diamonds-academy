package admin

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"strconv"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"github.com/diamondsacademy/diamonds/internal/auth"
	"github.com/diamondsacademy/diamonds/internal/days"
	"github.com/diamondsacademy/diamonds/internal/edusteps"
	"github.com/diamondsacademy/diamonds/internal/views/pages"
)

type Handler struct {
	SM       *scs.SessionManager
	DB       *sql.DB
	Days     *days.Repo
	EduSteps *edusteps.Repo
}

func New(sm *scs.SessionManager, db *sql.DB) *Handler {
	return &Handler{SM: sm, DB: db, Days: days.NewRepo(db), EduSteps: edusteps.NewRepo(db)}
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	users, err := listUsers(r.Context(), h.DB)
	if err != nil {
		http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Toplam adım sayısı
	totalSteps := h.EduSteps.Count(r.Context())

	// Her kullanıcı için ilerleme hesapla
	var rows []pages.AdminUserRow
	for _, u := range users {
		var done int
		_ = h.DB.QueryRowContext(r.Context(),
			`SELECT COUNT(DISTINCT day_no) FROM slot_completion WHERE user_id = ?`, u.ID).Scan(&done)

		rows = append(rows, pages.AdminUserRow{
			User:       u,
			StepsDone:  done,
			TotalSteps: totalSteps,
			HasCert:    done >= totalSteps && totalSteps > 0,
		})
	}

	usdRate := usdTryRate()
	totalUSD, dailyUSD, liveDS, totalCalls := deepseekCost(r.Context(), h.DB)
	render(w, r, pages.Admin(pages.AdminProps{
		Users:           users,
		TotalSteps:      totalSteps,
		UserRows:        rows,
		DeepseekCostTRY: totalUSD * usdRate,
		DSDailyCostTRY:  dailyUSD * usdRate,
		DSLiveUsers:     liveDS,
		DSTotalCalls:    totalCalls,
	}))
}

func usdTryRate() float64 {
	if v := os.Getenv("USD_TRY_RATE"); v != "" {
		if r, err := strconv.ParseFloat(v, 64); err == nil && r > 0 {
			return r
		}
	}
	return 32.0
}

func deepseekCost(ctx context.Context, db *sql.DB) (totalUSD float64, dailyUSD float64, liveUsers int, totalCalls int) {
	var prompt, completion int
	_ = db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0)
		 FROM deepseek_usage WHERE strftime('%Y-%m', created_at) = strftime('%Y-%m', 'now')`).Scan(&prompt, &completion)
	// DeepSeek pricing: $0.27/1M input, $1.10/1M output (deepseek-chat)
	const pricePrompt = 0.27 / 1000000
	const priceCompletion = 1.10 / 1000000
	totalUSD = float64(prompt)*pricePrompt + float64(completion)*priceCompletion

	var dprompt, dcompletion int
	_ = db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0)
		 FROM deepseek_usage WHERE date(created_at) = date('now')`).Scan(&dprompt, &dcompletion)
	dailyUSD = float64(dprompt)*pricePrompt + float64(dcompletion)*priceCompletion

	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM deepseek_usage
		 WHERE created_at > datetime('now', '-5 minutes')`).Scan(&liveUsers)
	_ = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM deepseek_usage
		 WHERE strftime('%Y-%m', created_at) = strftime('%Y-%m', 'now')`).Scan(&totalCalls)
	return
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
