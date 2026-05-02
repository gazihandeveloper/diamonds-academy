package days

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrNotFound = errors.New("day not found")

type Day struct {
	ID          int64
	DayNo       int
	Title       string
	Bullets     []string
	Description string
	VideoURL    string // legacy
	Published   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Video1URL string
	Video2URL string
	Video3URL string
	FilePath  string
	QuizText  string
	QuizJSON  string
}

type Repo struct{ DB *sql.DB }

func NewRepo(db *sql.DB) *Repo { return &Repo{DB: db} }

const dayCols = `id, day_no, title, bullets, description, video_url, published, created_at, updated_at,
	video1_url, video2_url, video3_url, file_path, quiz_text, quiz_json`

func (r *Repo) List(ctx context.Context) ([]Day, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT `+dayCols+` FROM days ORDER BY day_no ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Day
	for rows.Next() {
		d, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repo) GetByID(ctx context.Context, id int64) (*Day, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT `+dayCols+` FROM days WHERE id = ?`, id)
	d, err := scan(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (r *Repo) GetByDayNo(ctx context.Context, dayNo int) (*Day, error) {
	row := r.DB.QueryRowContext(ctx, `SELECT `+dayCols+` FROM days WHERE day_no = ?`, dayNo)
	d, err := scan(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

type Input struct {
	DayNo       int
	Title       string
	Bullets     []string
	Description string
	VideoURL    string
	Published   bool

	Video1URL string
	Video2URL string
	Video3URL string
	FilePath  string
	QuizText  string
	QuizJSON  string
}

func (r *Repo) Create(ctx context.Context, in Input) (int64, error) {
	bj, _ := json.Marshal(cleanBullets(in.Bullets))
	res, err := r.DB.ExecContext(ctx,
		`INSERT INTO days(day_no, title, bullets, description, video_url, published,
			video1_url, video2_url, video3_url, file_path, quiz_text, quiz_json)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		in.DayNo, strings.TrimSpace(in.Title), string(bj),
		strings.TrimSpace(in.Description), strings.TrimSpace(in.VideoURL),
		boolToInt(in.Published),
		strings.TrimSpace(in.Video1URL), strings.TrimSpace(in.Video2URL), strings.TrimSpace(in.Video3URL),
		strings.TrimSpace(in.FilePath), in.QuizText, in.QuizJSON,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Repo) Update(ctx context.Context, id int64, in Input) error {
	bj, _ := json.Marshal(cleanBullets(in.Bullets))
	res, err := r.DB.ExecContext(ctx,
		`UPDATE days
		    SET day_no=?, title=?, bullets=?, description=?, video_url=?, published=?,
		        video1_url=?, video2_url=?, video3_url=?, file_path=?, quiz_text=?, quiz_json=?,
		        updated_at=CURRENT_TIMESTAMP
		  WHERE id=?`,
		in.DayNo, strings.TrimSpace(in.Title), string(bj),
		strings.TrimSpace(in.Description), strings.TrimSpace(in.VideoURL),
		boolToInt(in.Published),
		strings.TrimSpace(in.Video1URL), strings.TrimSpace(in.Video2URL), strings.TrimSpace(in.Video3URL),
		strings.TrimSpace(in.FilePath), in.QuizText, in.QuizJSON,
		id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) Delete(ctx context.Context, id int64) error {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM days WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type scanner interface{ Scan(dest ...any) error }

func scan(s scanner) (Day, error) {
	var d Day
	var bullets string
	var pub int
	if err := s.Scan(&d.ID, &d.DayNo, &d.Title, &bullets, &d.Description,
		&d.VideoURL, &pub, &d.CreatedAt, &d.UpdatedAt,
		&d.Video1URL, &d.Video2URL, &d.Video3URL, &d.FilePath, &d.QuizText, &d.QuizJSON); err != nil {
		return Day{}, err
	}
	d.Published = pub == 1
	if bullets != "" {
		_ = json.Unmarshal([]byte(bullets), &d.Bullets)
	}
	return d, nil
}

func cleanBullets(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
