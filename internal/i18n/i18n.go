package i18n

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alexedwards/scs/v2"

	"github.com/diamondsacademy/diamonds/internal/session"
)

const (
	LocaleTR = "tr"
	LocaleEN = "en"
	LocaleBG = "bg"
)

const DefaultLocale = LocaleTR

var Locales = []string{LocaleTR, LocaleEN, LocaleBG}

func LocaleNext(current string) string {
	for i, l := range Locales {
		if l == current {
			return Locales[(i+1)%len(Locales)]
		}
	}
	return LocaleTR
}

func LocaleUpper(current string) string {
	return strings.ToUpper(current)
}

var (
	trData = defaultTR()
	enData = defaultEN()
	bgData = defaultBG()

	mu sync.RWMutex

	// Cached at startup — avoids syscall per request
	forcedLocale = os.Getenv("APP_LOCALE")

	// Package-level to avoid allocation per DetectFromHeader call
	supportedLocales = map[string]bool{LocaleTR: true, LocaleEN: true, LocaleBG: true}
)

type ctxKey struct{}

func ContextWithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, ctxKey{}, locale)
}

type themeCtxKey struct{}

func ContextWithTheme(ctx context.Context, theme string) context.Context {
	return context.WithValue(ctx, themeCtxKey{}, theme)
}

func ThemeFromContext(ctx context.Context) string {
	if t, ok := ctx.Value(themeCtxKey{}).(string); ok && t != "" {
		return t
	}
	return "light"
}

func FromContext(ctx context.Context) string {
	if l, ok := ctx.Value(ctxKey{}).(string); ok && l != "" {
		return l
	}
	return DefaultLocale
}

func DetectFromHeader(acceptLang string) string {
	if acceptLang == "" {
		return DefaultLocale
	}
	entries := strings.Split(strings.ToLower(acceptLang), ",")
	bestQ := -1.0
	bestLang := DefaultLocale
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.Split(entry, ";")
		langTag := strings.TrimSpace(parts[0])
		lang := langTag
		if idx := strings.Index(langTag, "-"); idx > 0 {
			lang = langTag[:idx]
		}
		q := 1.0
		if len(parts) > 1 {
			qStr := strings.TrimSpace(parts[1])
			if strings.HasPrefix(qStr, "q=") {
				fmt.Sscanf(qStr, "q=%f", &q)
			}
		}
		if supportedLocales[lang] && q > bestQ {
			bestQ = q
			bestLang = lang
		}
	}
	return bestLang
}

func Middleware(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isValid(forcedLocale) {
				sm.Put(r.Context(), session.KeyLocale, forcedLocale)
				ctx := ContextWithLocale(r.Context(), forcedLocale)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			locale := sm.GetString(r.Context(), session.KeyLocale)
			if locale == "" || !isValid(locale) {
				locale = DetectFromHeader(r.Header.Get("Accept-Language"))
				sm.Put(r.Context(), session.KeyLocale, locale)
			}
			ctx := ContextWithLocale(r.Context(), locale)

			theme := sm.GetString(r.Context(), session.KeyTheme)
			if theme == "" {
				theme = "light"
			}
			ctx = ContextWithTheme(ctx, theme)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isValid(l string) bool {
	return l == LocaleTR || l == LocaleEN || l == LocaleBG
}

func Load(path string) error {
	mu.Lock()
	defer mu.Unlock()

	for _, code := range []string{LocaleTR, LocaleEN, LocaleBG} {
		p := filepath.Join(path, code+".json")
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read %s: %w", p, err)
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		switch code {
		case LocaleTR:
			for k, v := range m {
				trData[k] = v
			}
		case LocaleEN:
			for k, v := range m {
				enData[k] = v
			}
		case LocaleBG:
			for k, v := range m {
				bgData[k] = v
			}
		}
	}
	return nil
}

func T(locale, key string, args ...any) string {
	mu.RLock()
	msg := enData[key]
	if locale == LocaleBG {
		msg = bgData[key]
	}
	if locale == LocaleTR || msg == "" {
		if m, ok := trData[key]; ok {
			msg = m
		}
	}
	mu.RUnlock()
	if msg == "" {
		return key
	}
	if len(args) > 0 {
		for i, a := range args {
			msg = strings.ReplaceAll(msg, fmt.Sprintf("{%d}", i), fmt.Sprintf("%v", a))
		}
		msg = strings.ReplaceAll(msg, "{n}", fmt.Sprintf("%v", args[0]))
	}
	return msg
}

func TC(ctx context.Context, key string, args ...any) string {
	return T(FromContext(ctx), key, args...)
}

func defaultTR() map[string]string { return map[string]string{} }
func defaultEN() map[string]string { return map[string]string{} }
func defaultBG() map[string]string { return map[string]string{} }
