package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/diamondsacademy/diamonds/internal/i18n"
	"github.com/diamondsacademy/diamonds/internal/transcript"
)

const cacheTTL = 1 * time.Hour

type cacheEntry struct {
	body      string
	expiresAt time.Time
}

var (
	vttCache   = map[string]cacheEntry{}
	vttCacheMu sync.RWMutex
)

func (h *Handler) CaptionsHandler(w http.ResponseWriter, r *http.Request) {
	videoID := r.URL.Query().Get("v")
	lang := r.URL.Query().Get("lang")
	if videoID == "" || (lang != "en" && lang != "bg" && lang != "tr") {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	cacheKey := videoID + ":" + lang

	vttCacheMu.RLock()
	entry, ok := vttCache[cacheKey]
	vttCacheMu.RUnlock()

	if ok && time.Now().Before(entry.expiresAt) {
		writeVTT(w, entry.body)
		return
	}

	// Check DB
	if h.DB != nil {
		repo := transcript.NewRepo(h.DB)
		t, err := repo.GetByVideoID(r.Context(), videoID, lang)
		if err == nil && t != nil && len(t.VTT) > 30 {
			writeVTT(w, t.VTT)
			return
		}
	}

	writeVTT(w, "WEBVTT\n\n")
}

func (h *Handler) SaveCaptionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)

	var req struct {
		VideoID string `json:"video_id"`
		VTT     string `json:"vtt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.VideoID == "" || req.VTT == "" {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}

	enVTT := translateVTT(req.VTT, i18n.LocaleTR, i18n.LocaleEN)
	bgVTT := translateVTT(req.VTT, i18n.LocaleTR, i18n.LocaleBG)

	vttCacheMu.Lock()
	vttCache[req.VideoID+":en"] = cacheEntry{body: enVTT, expiresAt: time.Now().Add(cacheTTL)}
	vttCache[req.VideoID+":bg"] = cacheEntry{body: bgVTT, expiresAt: time.Now().Add(cacheTTL)}
	vttCacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"cached": req.VideoID,
	})
}

func (h *Handler) FetchTranscriptHandler(w http.ResponseWriter, r *http.Request) {
	videoID := r.URL.Query().Get("v")
	if videoID == "" || len(videoID) < 6 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad video id"})
		return
	}

	vtt, err := transcript.FetchFromYouTube(videoID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"error": err.Error(), "video_id": videoID})
		return
	}

	repo := transcript.NewRepo(h.DB)
	if err := repo.Upsert(r.Context(), videoID, "tr", vtt); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	enVTT := translateVTT(vtt, i18n.LocaleTR, i18n.LocaleEN)
	bgVTT := translateVTT(vtt, i18n.LocaleTR, i18n.LocaleBG)
	_ = repo.Upsert(r.Context(), videoID, "en", enVTT)
	_ = repo.Upsert(r.Context(), videoID, "bg", bgVTT)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "video_id": videoID, "length": strconv.Itoa(len(vtt))})
}

func (h *Handler) SaveTranscriptHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		VideoID string `json:"video_id"`
		VTT     string `json:"vtt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.VideoID == "" || req.VTT == "" {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}

	repo := transcript.NewRepo(h.DB)
	if err := repo.Upsert(r.Context(), req.VideoID, "tr", req.VTT); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func writeVTT(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write([]byte(body))
}

func splitVTTCues(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	raw := strings.Split(body, "\n\n")
	var cues []string
	for _, c := range raw {
		c = strings.TrimSpace(c)
		if c != "" {
			cues = append(cues, c)
		}
	}
	return cues
}

func translateVTT(vtt, from, to string) string {
	vtt = strings.TrimSpace(vtt)
	if vtt == "" {
		return "WEBVTT\n\n"
	}
	lines := strings.SplitN(vtt, "\n\n", 2)
	header := "WEBVTT"
	body := vtt
	if len(lines) >= 2 && strings.HasPrefix(strings.ToUpper(lines[0]), "WEBVTT") {
		header = lines[0]
		body = lines[1]
	}

	blocks := splitVTTCues(body)
	var outBlocks []string
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		parts := strings.SplitN(block, "\n", 2)
		if len(parts) < 2 {
			outBlocks = append(outBlocks, block)
			continue
		}
		timestamp := strings.TrimSpace(parts[0])
		if !strings.Contains(timestamp, "-->") {
			outBlocks = append(outBlocks, block)
			continue
		}
		textLines := strings.Split(parts[1], "\n")
		translatedLines := make([]string, 0, len(textLines))
		for _, line := range textLines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			translated := i18n.TranslateText(line, from, to)
			translatedLines = append(translatedLines, translated)
		}
		if len(translatedLines) == 0 {
			outBlocks = append(outBlocks, block)
			continue
		}
		outBlocks = append(outBlocks, timestamp+"\n"+strings.Join(translatedLines, "\n"))
	}

	return header + "\n\n" + strings.Join(outBlocks, "\n\n") + "\n"
}
