package generator

import (
	"context"
	"testing"
)

const sampleHAR = `{
  "log": {
    "entries": [
      {"request": {"method": "GET", "url": "https://shop.example.co.id/api/v1/products", "headers": [{"name": "Accept", "value": "application/json"}]}},
      {"request": {"method": "POST", "url": "https://shop.example.co.id/api/v1/cart", "headers": [{"name": "Content-Type", "value": "application/json"}], "postData": {"mimeType": "application/json", "text": "{\"product_id\": 101, \"quantity\": 2}"}}},
      {"request": {"method": "GET", "url": "https://analytics.thirdparty.com/beacon?x=1", "headers": []}}
    ]
  }
}`

func TestImportHAR(t *testing.T) {
	sc, err := ImportHAR(context.Background(), []byte(sampleHAR), GeneratorConfig{})
	if err != nil {
		t.Fatalf("ImportHAR failed: %v", err)
	}
	if sc.BaseURL != "https://shop.example.co.id" {
		t.Errorf("expected base URL to be the first same-origin request's origin, got %s", sc.BaseURL)
	}
	// The third-party analytics beacon must be dropped (different origin).
	if len(sc.Steps) != 2 {
		t.Fatalf("expected 2 same-origin steps, got %d", len(sc.Steps))
	}
}

func TestImportHARRejectsEmpty(t *testing.T) {
	if _, err := ImportHAR(context.Background(), []byte(`{"log":{"entries":[]}}`), GeneratorConfig{}); err == nil {
		t.Error("expected error for HAR with no entries")
	}
}

func TestImportHARLocalizesIndonesian(t *testing.T) {
	sc, err := ImportHAR(context.Background(), []byte(sampleHAR), GeneratorConfig{Locale: "id"})
	if err != nil {
		t.Fatalf("ImportHAR failed: %v", err)
	}
	found := false
	for _, step := range sc.Steps {
		if step.Request.Body != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one step to retain its JSON body after localization")
	}
}
