// Package i18n is the server-side counterpart to the dashboard's I18nContext (web/src/context):
// it embeds the same three locale JSON files (locales/*.json — en, id, jp) the frontend ships, so
// that server-generated, user-facing text (API error messages via T()) is translated consistently
// with the UI instead of always falling back to English. RequestLang resolves which language a
// given HTTP request wants from its "lang" query parameter or Accept-Language header, normalizing
// both "ja" and "jp" to the same "jp" catalog the frontend also treats as canonical (see the web
// i18n docs/comments for why "jp" — not the ISO 639-1 "ja" — ended up as the internal code).
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localesFS embed.FS

var (
	mu          sync.RWMutex
	catalog     = make(map[string]map[string]string)
	supported   = []string{"en", "id", "jp"}
	defaultLang = "en"
)

func init() {
	loadLocales()
}

func loadLocales() {
	mu.Lock()
	defer mu.Unlock()

	for _, lang := range []string{"en", "id", "jp"} {
		path := fmt.Sprintf("locales/%s.json", lang)
		data, err := localesFS.ReadFile(path)
		if err != nil {
			// The embedded FS is compiled in, so this can only fail if locales/*.json's
			// go:embed glob stopped matching a file the code still expects — a build-time
			// packaging mistake, not a runtime condition. T() falls back to English silently
			// otherwise, so this is logged to give any translator/build a trail to notice.
			log.Printf("[Oshimai] i18n: failed to read embedded locale %q: %v", path, err)
			continue
		}

		var nested map[string]interface{}
		if err := json.Unmarshal(data, &nested); err != nil {
			log.Printf("[Oshimai] i18n: locale %q contains malformed JSON, falling back to %q for it: %v", path, defaultLang, err)
			continue
		}

		flat := make(map[string]string)
		flatten("", nested, flat)

		catalog[lang] = flat
	}
}

func flatten(prefix string, current map[string]interface{}, out map[string]string) {
	for k, v := range current {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			out[key] = val
		case map[string]interface{}:
			flatten(key, val, out)
		}
	}
}

// SupportedLanguages returns the list of official language codes.
func SupportedLanguages() []string {
	return supported
}

// NormalizeLang normalizes input language code to en, id, or jp.
func NormalizeLang(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if strings.HasPrefix(code, "id") {
		return "id"
	}
	if strings.HasPrefix(code, "jp") || strings.HasPrefix(code, "ja") {
		return "jp"
	}
	if strings.HasPrefix(code, "en") {
		return "en"
	}
	return defaultLang
}

// RequestLang extracts and normalizes the target language from an HTTP request.
// It checks the query parameter "lang", followed by the "Accept-Language" header.
func RequestLang(r *http.Request) string {
	if r == nil {
		return defaultLang
	}
	if q := r.URL.Query().Get("lang"); q != "" {
		return NormalizeLang(q)
	}
	if al := r.Header.Get("Accept-Language"); al != "" {
		parts := strings.Split(al, ",")
		if len(parts) > 0 {
			tag := strings.TrimSpace(strings.Split(parts[0], ";")[0])
			return NormalizeLang(tag)
		}
	}
	return defaultLang
}

// T translates a key into the given language with optional fmt.Sprintf arguments.
// If the key is not found in the given language, it falls back to English, then returns the key itself.
func T(lang, key string, args ...interface{}) string {
	mu.RLock()
	defer mu.RUnlock()

	normalized := NormalizeLang(lang)
	dict, ok := catalog[normalized]
	if !ok {
		dict = catalog[defaultLang]
	}

	msg, found := dict[key]
	if !found {
		// Fallback to default
		if defDict, defOk := catalog[defaultLang]; defOk {
			msg, found = defDict[key]
		}
	}

	if !found {
		msg = key
	}

	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// GetCatalog returns the full dictionary for the given language.
func GetCatalog(lang string) map[string]string {
	mu.RLock()
	defer mu.RUnlock()

	normalized := NormalizeLang(lang)
	if dict, ok := catalog[normalized]; ok {
		cpy := make(map[string]string, len(dict))
		for k, v := range dict {
			cpy[k] = v
		}
		return cpy
	}
	return catalog[defaultLang]
}
