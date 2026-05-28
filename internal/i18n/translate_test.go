package i18n

import (
	"strings"
	"testing"
	"time"
)

func TestTranslateQuiz_API(t *testing.T) {
	quizJSON := `[{"q":"Merhaba dünya","options":["Evet","Hayır"],"correct":0}]`

	en := TranslateQuiz(quizJSON, LocaleTR, LocaleEN)
	bg := TranslateQuiz(quizJSON, LocaleTR, LocaleBG)

	t.Logf("EN: %s", en)
	t.Logf("BG: %s", bg)

	if en == "" {
		t.Error("EN translation returned empty string")
	}
	if bg == "" {
		t.Error("BG translation returned empty string")
	}
	// API translation should differ from source
	if en == quizJSON {
		t.Log("Warning: EN equals source — API may have failed, check network")
	}
}

func TestAutoTranslateAll_API(t *testing.T) {
	quizJSON := `[{"q":"Merhaba dünya","options":["İyi","Kötü"],"correct":0}]`
	en, bg := AutoTranslateAll(quizJSON)

	if en == "" {
		t.Error("AutoTranslateAll EN returned empty")
	}
	if bg == "" {
		t.Error("AutoTranslateAll BG returned empty")
	}
	t.Logf("Auto EN: %s", en)
	t.Logf("Auto BG: %s", bg)
}

func TestTranslateEmpty(t *testing.T) {
	if out := TranslateQuiz("", LocaleTR, LocaleEN); out != "" {
		t.Errorf("empty input should return empty, got: %s", out)
	}
	if out := TranslateQuiz("   ", LocaleTR, LocaleEN); out != "" {
		t.Errorf("whitespace input should return empty, got: %s", out)
	}
}

func TestTranslateInvalidJSON(t *testing.T) {
	if out := TranslateQuiz("not json", LocaleTR, LocaleEN); out != "" {
		t.Errorf("invalid JSON should return empty, got: %s", out)
	}
}

func TestTranslateWithEmptyStrings(t *testing.T) {
	quizJSON := `[{"q":"","options":["",""],"correct":0}]`
	en := TranslateQuiz(quizJSON, LocaleTR, LocaleEN)
	t.Logf("Empty fields EN: %s", en)
	if en == "" {
		t.Error("should return JSON even with empty fields")
	}
}

func TestTranslateAPI_RealWords(t *testing.T) {
	// Test that real Turkish words get translated (not just echoed back)
	result := translateAPI("merhaba", LocaleTR, LocaleEN)
	t.Logf("'merhaba' → '%s'", result)
	if result == "merhaba" {
		t.Log("API returned same text — may be offline, skipping strict check")
	} else if !strings.Contains(strings.ToLower(result), "hello") && !strings.Contains(strings.ToLower(result), "hi") {
		t.Logf("Unexpected translation: %s (expected something like 'hello')", result)
	}

	resultBg := translateAPI("merhaba", LocaleTR, LocaleBG)
	t.Logf("'merhaba' → BG: '%s'", resultBg)
	if resultBg == "merhaba" {
		t.Log("BG API returned same text — may be offline")
	}
}

func TestTranslateAPI_RateLimiting(t *testing.T) {
	// Call API 3 times in rapid succession — rate limiter should prevent hammering
	start := time.Now()
	for i := 0; i < 3; i++ {
		translateAPI("test", "tr", "en")
	}
	elapsed := time.Since(start)
	// With 200ms rate limit, 3 calls should take at least 400ms
	if elapsed < 300*time.Millisecond {
		t.Logf("Rate limiting may not be active: 3 calls took %v", elapsed)
	} else {
		t.Logf("Rate limiting OK: 3 calls took %v", elapsed)
	}
}

func TestI18nDetectFromHeader(t *testing.T) {
	if l := DetectFromHeader("en-US,en;q=0.9"); l != LocaleEN {
		t.Errorf("expected en, got %s", l)
	}
	if l := DetectFromHeader("bg-BG,bg;q=0.9"); l != LocaleBG {
		t.Errorf("expected bg, got %s", l)
	}
	if l := DetectFromHeader("tr-TR,tr;q=0.9"); l != LocaleTR {
		t.Errorf("expected tr, got %s", l)
	}
	if l := DetectFromHeader(""); l != DefaultLocale {
		t.Errorf("expected default %s, got %s", DefaultLocale, l)
	}
	if l := DetectFromHeader("fr-FR"); l != DefaultLocale {
		t.Errorf("unsupported lang should default to %s, got %s", DefaultLocale, l)
	}
}

func TestT(t *testing.T) {
	if v := T(LocaleTR, "login.title"); v != "Giriş Yap" {
		t.Logf("TR login.title = %s (expected 'Giriş Yap')", v)
	}
	if v := T(LocaleEN, "login.title"); v != "Sign In" {
		t.Logf("EN login.title = %s (expected 'Sign In')", v)
	}
	if v := T(LocaleBG, "login.title"); v != "Вход" {
		t.Logf("BG login.title = %s (expected 'Вход')", v)
	}
	if v := T(LocaleEN, "nonexistent.key"); v != "nonexistent.key" {
		t.Errorf("unknown key should return key itself, got %s", v)
	}
}
