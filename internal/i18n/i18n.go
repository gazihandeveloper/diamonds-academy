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

var (
	trData = defaultTR()
	enData = defaultEN()
	bgData = defaultBG()

	mu sync.RWMutex
)

type ctxKey struct{}

func ContextWithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, ctxKey{}, locale)
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
	supported := map[string]bool{LocaleTR: true, LocaleEN: true, LocaleBG: true}
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
		if supported[lang] && q > bestQ {
			bestQ = q
			bestLang = lang
		}
	}
	return bestLang
}

func Middleware(sm *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			forced := os.Getenv("APP_LOCALE")
			if isValid(forced) {
				sm.Put(r.Context(), session.KeyLocale, forced)
				ctx := ContextWithLocale(r.Context(), forced)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			locale := sm.GetString(r.Context(), session.KeyLocale)
			if locale == "" || !isValid(locale) {
				locale = DetectFromHeader(r.Header.Get("Accept-Language"))
				sm.Put(r.Context(), session.KeyLocale, locale)
			}
			ctx := ContextWithLocale(r.Context(), locale)
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
