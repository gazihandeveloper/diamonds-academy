package transcript

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Transcript struct {
	ID        int64     `json:"id"`
	VideoID   string    `json:"video_id"`
	Language  string    `json:"language"`
	VTT       string    `json:"vtt"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Repo struct {
	DB *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{DB: db}
}

func (r *Repo) GetByVideoID(ctx context.Context, videoID, lang string) (*Transcript, error) {
	row := r.DB.QueryRowContext(ctx,
		`SELECT id, video_id, language, vtt, created_at, updated_at FROM transcripts WHERE video_id = ? AND language = ?`,
		videoID, lang)
	var t Transcript
	err := row.Scan(&t.ID, &t.VideoID, &t.Language, &t.VTT, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repo) Upsert(ctx context.Context, videoID, lang, vtt string) error {
	_, err := r.DB.ExecContext(ctx,
		`INSERT INTO transcripts(video_id, language, vtt, updated_at) VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(video_id, language) DO UPDATE SET vtt = excluded.vtt, updated_at = CURRENT_TIMESTAMP`,
		videoID, lang, vtt)
	return err
}

type youtubeCue struct {
	Text     string  `json:"text"`
	Duration float64 `json:"duration"`
	Start    float64 `json:"start"`
}

func FetchFromYouTube(videoID string) (string, error) {
	url := fmt.Sprintf("https://youtubetranscript.com/?v=%s&format=json", videoID)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetch transcript: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("youtubetranscript returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return "", fmt.Errorf("read transcript: %w", err)
	}

	var cues []youtubeCue
	if err := json.Unmarshal(body, &cues); err != nil {
		return "", fmt.Errorf("parse transcript json: %w", err)
	}

	if len(cues) == 0 {
		return "WEBVTT\n\n", nil
	}

	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, c := range cues {
		start := formatVTTTime(c.Start)
		end := formatVTTTime(c.Start + c.Duration)
		text := strings.TrimSpace(c.Text)
		text = strings.ReplaceAll(text, "\n", " ")
		text = strings.ReplaceAll(text, "\r", "")
		if text == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("%s --> %s\n%s\n\n", start, end, text))
	}
	return b.String(), nil
}

func formatVTTTime(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}
