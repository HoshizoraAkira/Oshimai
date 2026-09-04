package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSupportedLanguages(t *testing.T) {
	langs := SupportedLanguages()
	if len(langs) != 3 {
		t.Fatalf("expected 3 languages, got %d", len(langs))
	}
}

func TestT(t *testing.T) {
	tests := []struct {
		lang     string
		key      string
		args     []interface{}
		expected string
	}{
		{"en", "server.internal_error", nil, "Internal server error"},
		{"id", "server.internal_error", nil, "Terjadi kesalahan internal server"},
		{"jp", "server.internal_error", nil, "内部サーバーエラーが発生しました"},
		{"en", "server.run_not_found", []interface{}{"run-123"}, "Run \"run-123\" not found"},
		{"id", "server.run_not_found", []interface{}{"run-123"}, "Uji jalan (run) \"run-123\" tidak ditemukan"},
		{"jp", "server.run_not_found", []interface{}{"run-123"}, "テスト実行 \"run-123\" が見つかりません"},
	}

	for _, tt := range tests {
		got := T(tt.lang, tt.key, tt.args...)
		if got != tt.expected {
			t.Errorf("T(%s, %s) = %q; want %q", tt.lang, tt.key, got, tt.expected)
		}
	}
}

func TestRequestLang(t *testing.T) {
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/test?lang=id", nil)
	if got := RequestLang(req1); got != "id" {
		t.Errorf("expected 'id', got %q", got)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	req2.Header.Set("Accept-Language", "ja,en-US;q=0.9,en;q=0.8")
	if got := RequestLang(req2); got != "jp" {
		t.Errorf("expected 'jp', got %q", got)
	}

	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	if got := RequestLang(req3); got != "en" {
		t.Errorf("expected default 'en', got %q", got)
	}
}
