package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
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
			continue
		}

		var nested map[string]interface{}
		if err := json.Unmarshal(data, &nested); err != nil {
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
