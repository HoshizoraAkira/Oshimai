package generator

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLocalizeIndonesian(t *testing.T) {
	input := `{"email": "user@example.com", "phone_number": "555-1234", "full_name": "John Doe"}`
	out := LocalizeIndonesian(input)

	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("localized output is not valid JSON: %v", err)
	}

	if decoded["email"] != "pelanggan@contoh.co.id" {
		t.Errorf("expected localized email, got %v", decoded["email"])
	}
	phone, _ := decoded["phone_number"].(string)
	if !strings.HasPrefix(phone, "+62812") {
		t.Errorf("expected +62 phone number, got %v", phone)
	}
	name, _ := decoded["full_name"].(string)
	if name == "John Doe" {
		t.Error("expected full_name to be localized away from the original en-US value")
	}
}

func TestLocalizeIndonesianLeavesUnknownFieldsAlone(t *testing.T) {
	input := `{"sku": "ABC-123", "price": 99.5}`
	out := LocalizeIndonesian(input)
	var decoded map[string]any
	json.Unmarshal([]byte(out), &decoded)
	if decoded["sku"] != "ABC-123" {
		t.Errorf("expected sku field to remain untouched, got %v", decoded["sku"])
	}
}

func TestLocalizeIndonesianPassesThroughInvalidJSON(t *testing.T) {
	if got := LocalizeIndonesian("not json"); got != "not json" {
		t.Errorf("expected unparseable input returned unchanged, got %q", got)
	}
	if got := LocalizeIndonesian(""); got != "" {
		t.Errorf("expected empty input returned unchanged, got %q", got)
	}
}
