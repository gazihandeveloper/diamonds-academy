package progress

import (
	"context"
	"database/sql"
	"time"
)

type Repo struct{ db *sql.DB }

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

type Entry struct {
	ID             int64
	UserID         int64
	DayNo          int
	Slot           string
	Position       float64
	Duration       float64
	SecondsWatched float64
	Percent        float64
	Completed      bool
	UpdatedAt      time.Time
}

func (r *Repo) Upsert(ctx context.Context, e Entry, secondsDelta float64) error {
	completed := 0
	if e.Percent >= 90 {
		completed = 1
	}
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO watch_progress
			(user_id, day_no, slot, position, duration, seconds_watched, percent, completed, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(user_id, day_no, slot) DO UPDATE SET
			position        = excluded.position,
			duration        = CASE WHEN excluded.duration > 0 THEN excluded.duration ELSE watch_progress.duration END,
			seconds_watched = watch_progress.seconds_watched + ?,
			percent         = CASE WHEN excluded.percent > watch_progress.percent THEN excluded.percent ELSE watch_progress.percent END,
			completed       = CASE WHEN excluded.percent >= 90 OR watch_progress.completed = 1 THEN 1 ELSE 0 END,
			updated_at      = CURRENT_TIMESTAMP
	`, e.UserID, e.DayNo, e.Slot, e.Position, e.Duration, secondsDelta, e.Percent, completed, secondsDelta); err != nil {
		return err
	}
	if completed == 1 {
		return r.MarkComplete(ctx, e.UserID, e.DayNo, e.Slot)
	}
	return nil
}

func (r *Repo) ListForUserDay(ctx context.Context, userID int64, dayNo int) ([]Entry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, day_no, slot, position, duration, seconds_watched, percent, completed, updated_at
		FROM watch_progress WHERE user_id = ? AND day_no = ?`, userID, dayNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var c int
		if err := rows.Scan(&e.ID, &e.UserID, &e.DayNo, &e.Slot, &e.Position, &e.Duration, &e.SecondsWatched, &e.Percent, &c, &e.UpdatedAt); err != nil {
			return nil, err
		}
		e.Completed = c == 1
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repo) MarkComplete(ctx context.Context, userID int64, dayNo int, slot string) error {
	if userID == 0 {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO slot_completion(user_id, day_no, slot) VALUES (?, ?, ?)`,
		userID, dayNo, slot)
	return err
}

func (r *Repo) CompletedSlots(ctx context.Context, userID int64, dayNo int) (map[string]bool, error) {
	out := map[string]bool{}
	if userID == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT slot FROM slot_completion WHERE user_id = ? AND day_no = ?`, userID, dayNo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out[s] = true
	}
	return out, rows.Err()
}

// AllCompletedSlots returns all completed (dayNo -> slot -> true) for a user across all days.
func (r *Repo) AllCompletedSlots(ctx context.Context, userID int64) (map[int]map[string]bool, error) {
	out := map[int]map[string]bool{}
	if userID == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT day_no, slot FROM slot_completion WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d int
		var s string
		if err := rows.Scan(&d, &s); err != nil {
			return nil, err
		}
		if out[d] == nil {
			out[d] = map[string]bool{}
		}
		out[d][s] = true
	}
	return out, rows.Err()
}

// AnySlotCompleted returns true if any slot in the given day is completed.
func (r *Repo) AnySlotCompleted(ctx context.Context, userID int64, dayNo int) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	row := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM slot_completion WHERE user_id = ? AND day_no = ?`, userID, dayNo)
	var n int
	if err := row.Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *Repo) QuizCompleted(ctx context.Context, userID int64, dayNo int) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM slot_completion
		WHERE user_id = ? AND day_no = ? AND slot = 'quiz'`, userID, dayNo)
	var n int
	if err := row.Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *Repo) CompletedQuizzes(ctx context.Context, userID int64) (map[int]bool, error) {
	out := map[int]bool{}
	if userID == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT day_no FROM slot_completion
		WHERE user_id = ? AND slot = 'quiz'`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d int
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out[d] = true
	}
	return out, rows.Err()
}

func (r *Repo) DayCompleted(ctx context.Context, userID int64, dayNo int) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT slot) FROM slot_completion
		WHERE user_id = ? AND day_no = ? AND slot IN ('l1','l2','l3','file','quiz')`, userID, dayNo)
	var n int
	if err := row.Scan(&n); err != nil {
		return false, err
	}
	return n >= 5, nil
}

func (r *Repo) CompletedDays(ctx context.Context, userID int64) (map[int]bool, error) {
	out := map[int]bool{}
	if userID == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT day_no FROM slot_completion
		WHERE user_id = ? AND slot IN ('l1','l2','l3','file','quiz')
		GROUP BY day_no
		HAVING COUNT(DISTINCT slot) >= 5`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d int
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out[d] = true
	}
	return out, rows.Err()
}

// CompletedDaysAt: her tamamlanmış eğitim için, 5 slotun tamamlandığı
// son zaman (MAX(completed_at)). 24-saat kilit sayacının başlangıcı.
func (r *Repo) CompletedDaysAt(ctx context.Context, userID int64) (map[int]time.Time, error) {
	out := map[int]time.Time{}
	if userID == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT day_no, MAX(completed_at) FROM slot_completion
		WHERE user_id = ? AND slot IN ('l1','l2','l3','file','quiz')
		GROUP BY day_no
		HAVING COUNT(DISTINCT slot) >= 5`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d int
		var t time.Time
		if err := rows.Scan(&d, &t); err != nil {
			return nil, err
		}
		out[d] = t
	}
	return out, rows.Err()
}

type UserStat struct {
	UserID         int64
	Email          string
	Name           string
	TotalSeconds   float64
	DaysCompleted  int
	SlotsCompleted int
	LastSeen       sql.NullTime
}

func (r *Repo) UserStats(ctx context.Context) ([]UserStat, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			u.id, u.email, u.name,
			COALESCE(SUM(wp.seconds_watched), 0),
			(SELECT COUNT(*) FROM slot_completion sc WHERE sc.user_id = u.id),
			MAX(wp.updated_at)
		FROM users u
		LEFT JOIN watch_progress wp ON wp.user_id = u.id
		GROUP BY u.id, u.email, u.name
		ORDER BY 4 DESC, u.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserStat
	for rows.Next() {
		var s UserStat
		if err := rows.Scan(&s.UserID, &s.Email, &s.Name, &s.TotalSeconds, &s.SlotsCompleted, &s.LastSeen); err != nil {
			return nil, err
		}
		row := r.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM (
				SELECT day_no FROM slot_completion
				WHERE user_id = ? AND slot IN ('l1','l2','l3','file','quiz')
				GROUP BY day_no
				HAVING COUNT(DISTINCT slot) >= 5
			)`, s.UserID)
		_ = row.Scan(&s.DaysCompleted)
		out = append(out, s)
	}
	return out, rows.Err()
}
