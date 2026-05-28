package quiz

import (
	"encoding/json"
	"strings"
)

// Question — admin tarafından quiz_json olarak girilen tek soru.
// JSON formatı:
//   [
//     { "q": "Soru?", "options": ["A","B","C"], "correct": 1 },
//     ...
//   ]
type Question struct {
	Q       string   `json:"q"`
	Options []string `json:"options"`
	Correct int      `json:"correct"` // 0-tabanlı doğru index
}

const (
	rawKeyTR = "tr"
	rawKeyEN = "en"
	rawKeyBG = "bg"
)

// ParseForLocale picks the correct quiz JSON based on locale.
// rawTR is always required (Turkish, the primary content).
// rawEN and rawBG may be empty — if empty, falls back to Turkish.
func ParseForLocale(rawTR, rawEN, rawBG, locale string) []Question {
	raw := rawTR
	switch locale {
	case rawKeyEN:
		if rawEN != "" {
			raw = rawEN
		}
	case rawKeyBG:
		if rawBG != "" {
			raw = rawBG
		}
	}
	return Parse(raw)
}

func Parse(raw string) []Question {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var qs []Question
	if err := json.Unmarshal([]byte(raw), &qs); err != nil {
		return nil
	}
	out := make([]Question, 0, len(qs))
	for _, q := range qs {
		q.Q = strings.TrimSpace(q.Q)
		if q.Q == "" || len(q.Options) < 2 {
			continue
		}
		if q.Correct < 0 || q.Correct >= len(q.Options) {
			continue
		}
		out = append(out, q)
	}
	return out
}

// Grade returns (correctCount, total).
// answers: kullanıcının verdiği cevaplar (index), len(answers) >= len(qs) olmalı.
func Grade(qs []Question, answers []int) (int, int) {
	correct := 0
	for i, q := range qs {
		if i >= len(answers) {
			break
		}
		if answers[i] == q.Correct {
			correct++
		}
	}
	return correct, len(qs)
}
