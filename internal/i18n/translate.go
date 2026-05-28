package i18n

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxQuizInputLen    = 256 << 10 // 256 KB
	translateTimeout   = 10 * time.Second
	translateRateLimit = 500 * time.Millisecond // polite to Google
)

const googleTranslateURL = "https://translate.googleapis.com/translate_a/single"

var lastRequest time.Time

type quizItem struct {
	Q       string   `json:"q"`
	Options []string `json:"options"`
	Correct int      `json:"correct"`
}

// translateAPI calls Google Translate's free endpoint.
// Returns translated text, or original text on any error.
func translateAPI(text, from, to string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}

	// Rate limit
	if d := time.Since(lastRequest); d < translateRateLimit {
		time.Sleep(translateRateLimit - d)
	}

	params := url.Values{}
	params.Set("client", "gtx")
	params.Set("sl", from)
	params.Set("tl", to)
	params.Set("dt", "t")
	params.Set("q", text)

	client := &http.Client{Timeout: translateTimeout}
	resp, err := client.Get(googleTranslateURL + "?" + params.Encode())
	lastRequest = time.Now()
	if err != nil {
		return text
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128<<10)) // 128KB max

	// Google Translate returns: [[["translated","original",...]],...]
	// We need to parse this nested JSON structure
	translated := parseGoogleResponse(body)
	if translated == "" {
		return text
	}
	return translated
}

// parseGoogleResponse extracts translated text from Google's nested response.
// Format: [[["Hello world","Merhaba dünya",null,null,10]],null,"tr",...]
func parseGoogleResponse(body []byte) string {
	var resp []interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ""
	}
	if len(resp) == 0 {
		return ""
	}
	// First element is the translation array
	first, ok := resp[0].([]interface{})
	if !ok || len(first) == 0 {
		return ""
	}
	// Each translation entry: ["translated","original",null,null,score]
	entry, ok := first[0].([]interface{})
	if !ok || len(entry) == 0 {
		return ""
	}
	translated, ok := entry[0].(string)
	if !ok {
		return ""
	}
	return translated
}

// TranslateQuiz translates quiz JSON using Google Translate.
func TranslateQuiz(jsonStr string, fromLocale, toLocale string) string {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" || len(jsonStr) > maxQuizInputLen {
		return ""
	}
	var questions []quizItem
	if err := json.Unmarshal([]byte(jsonStr), &questions); err != nil {
		return ""
	}
	if len(questions) > 500 {
		return ""
	}
	out := make([]quizItem, 0, len(questions))
	for _, qi := range questions {
		qi.Q = translateAPI(qi.Q, fromLocale, toLocale)
		if qi.Correct < 0 || qi.Correct >= len(qi.Options) {
			continue
		}
		opts := make([]string, len(qi.Options))
		for j, opt := range qi.Options {
			opts[j] = translateAPI(opt, fromLocale, toLocale)
		}
		qi.Options = opts
		out = append(out, qi)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return jsonStr
	}
	return string(b)
}

// AutoTranslateAll produces EN and BG translations via Google Translate.
func AutoTranslateAll(quizJSON string) (en, bg string) {
	quizJSON = strings.TrimSpace(quizJSON)
	if quizJSON == "" || len(quizJSON) > maxQuizInputLen {
		return "", ""
	}
	var questions []quizItem
	if err := json.Unmarshal([]byte(quizJSON), &questions); err != nil {
		return "", ""
	}
	if len(questions) > 500 {
		return "", ""
	}

	enQuestions := translateQuestions(questions, LocaleTR, LocaleEN)
	bgQuestions := translateQuestions(questions, LocaleTR, LocaleBG)

	enB, err := json.Marshal(enQuestions)
	if err != nil {
		return "", ""
	}
	bgB, err := json.Marshal(bgQuestions)
	if err != nil {
		return "", ""
	}
	return string(enB), string(bgB)
}

func translateQuestions(qs []quizItem, from, to string) []quizItem {
	out := make([]quizItem, 0, len(qs))
	for _, qi := range qs {
		qi.Q = translateAPI(qi.Q, from, to)
		if qi.Correct < 0 || qi.Correct >= len(qi.Options) {
			continue
		}
		opts := make([]string, len(qi.Options))
		for j, opt := range qi.Options {
			opts[j] = translateAPI(opt, from, to)
		}
		qi.Options = opts
		out = append(out, qi)
	}
	return out
}

// translateText kept for test compatibility.
func translateText(text, from, to string) string {
	return translateAPI(text, from, to)
}

// TranslateText is the public API for translating a single text string.
func TranslateText(text, from, to string) string {
	return translateAPI(text, from, to)
}

// TranslateBullets translates a slice of strings.
func TranslateBullets(bullets []string, from, to string) []string {
	if len(bullets) == 0 {
		return nil
	}
	out := make([]string, len(bullets))
	for i, b := range bullets {
		out[i] = translateAPI(b, from, to)
	}
	return out
}

// TranslateDayContent translates title, bullets, and description for a day.
// Returns (titleEN, titleBG, bulletsEN, bulletsBG, descEN, descBG).
func TranslateDayContent(title string, bullets []string, description string) (
	titleEN, titleBG string,
	bulletsEN, bulletsBG []string,
	descEN, descBG string,
) {
	titleEN = translateAPI(title, LocaleTR, LocaleEN)
	titleBG = translateAPI(title, LocaleTR, LocaleBG)
	bulletsEN = TranslateBullets(bullets, LocaleTR, LocaleEN)
	bulletsBG = TranslateBullets(bullets, LocaleTR, LocaleBG)
	descEN = translateAPI(description, LocaleTR, LocaleEN)
	descBG = translateAPI(description, LocaleTR, LocaleBG)
	return
}

func init() {
	lastRequest = time.Now().Add(-translateRateLimit)
}
