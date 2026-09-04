package generator

import (
	"context"
	"strings"
	"testing"
)

const sampleInsomnia = `{
  "resources": [
    {
      "_id": "req_2", "_type": "request", "parentId": "fld_1", "name": "Checkout",
      "method": "POST", "url": "https://api.toko.co.id/v1/checkout",
      "metaSortKey": 2,
      "headers": [{"name": "Content-Type", "value": "application/json"}],
      "body": {"mimeType": "application/json", "text": "{\"cart_id\": \"{{ _.cart_id }}\"}"}
    },
    {
      "_id": "req_1", "_type": "request", "parentId": "fld_1", "name": "Login",
      "method": "POST", "url": "https://api.toko.co.id/v1/login",
      "metaSortKey": 1,
      "headers": [],
      "body": {"mimeType": "application/json", "text": "{\"user\": \"demo\"}"}
    },
    {"_id": "fld_1", "_type": "request_group", "name": "Checkout Flow"}
  ]
}`

func TestImportInsomniaCollection(t *testing.T) {
	sc, err := ImportInsomniaCollection(context.Background(), []byte(sampleInsomnia), GeneratorConfig{})
	if err != nil {
		t.Fatalf("ImportInsomniaCollection failed: %v", err)
	}
	if len(sc.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(sc.Steps))
	}

	// metaSortKey must order Login (1) before Checkout (2), regardless of array order.
	first := sc.Steps[sc.InitialStepID]
	if !strings.Contains(first.Request.Path, "login") {
		t.Errorf("expected the lower metaSortKey request first, got initial step path %q", first.Request.Path)
	}

	var sawInterpolated bool
	for _, step := range sc.Steps {
		if strings.Contains(step.Request.Body, "${cart_id}") {
			sawInterpolated = true
		}
	}
	if !sawInterpolated {
		t.Error("expected Insomnia {{ _.cart_id }} to be rewritten to ${cart_id}")
	}
}

func TestImportInsomniaCollectionRejectsEmpty(t *testing.T) {
	if _, err := ImportInsomniaCollection(context.Background(), []byte(`{"resources":[]}`), GeneratorConfig{}); err == nil {
		t.Error("expected error for an export with no requests")
	}
}
