package generator

import (
	"context"
	"strings"
	"testing"
)

const samplePostman = `{
  "info": {"name": "E-Commerce Flow"},
  "item": [
    {
      "name": "Login",
      "request": {
        "method": "POST",
        "header": [{"key": "Content-Type", "value": "application/json"}],
        "body": {"mode": "raw", "raw": "{\"username\": \"demo\", \"password\": \"secret\"}"},
        "url": {"raw": "https://api.toko.co.id/v1/login", "host": ["api", "toko", "co", "id"], "path": ["v1", "login"]}
      }
    },
    {
      "name": "Checkout Folder",
      "item": [
        {
          "name": "Create Order",
          "request": {
            "method": "POST",
            "header": [],
            "body": {"mode": "raw", "raw": "{\"cart_id\": \"{{cart_id}}\"}"},
            "url": {"raw": "https://api.toko.co.id/v1/orders"}
          }
        }
      ]
    }
  ]
}`

func TestImportPostmanCollection(t *testing.T) {
	sc, err := ImportPostmanCollection(context.Background(), []byte(samplePostman), GeneratorConfig{})
	if err != nil {
		t.Fatalf("ImportPostmanCollection failed: %v", err)
	}
	if len(sc.Steps) != 2 {
		t.Fatalf("expected 2 flattened steps (1 top-level + 1 nested), got %d", len(sc.Steps))
	}
	if sc.Name != "E-Commerce Flow" {
		t.Errorf("expected scenario name from collection info, got %q", sc.Name)
	}

	var sawInterpolated bool
	for _, step := range sc.Steps {
		if step.Request.Body != "" && strings.Contains(step.Request.Body, "${cart_id}") {
			sawInterpolated = true
		}
	}
	if !sawInterpolated {
		t.Error("expected Postman {{cart_id}} to be rewritten to ${cart_id}")
	}
}

func TestImportPostmanCollectionRejectsEmpty(t *testing.T) {
	if _, err := ImportPostmanCollection(context.Background(), []byte(`{"info":{"name":"x"},"item":[]}`), GeneratorConfig{}); err == nil {
		t.Error("expected error for collection with no items")
	}
}
