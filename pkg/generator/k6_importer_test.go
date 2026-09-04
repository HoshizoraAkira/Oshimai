package generator

import (
	"context"
	"testing"
)

const sampleK6Script = `
import http from 'k6/http';

export default function () {
  http.get('https://api.toko.co.id/v1/items');
  http.post('https://api.toko.co.id/v1/cart', JSON.stringify({item_id: 5}));
}
`

func TestImportK6Script(t *testing.T) {
	sc, err := ImportK6Script(context.Background(), sampleK6Script, GeneratorConfig{})
	if err != nil {
		t.Fatalf("ImportK6Script failed: %v", err)
	}
	if len(sc.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(sc.Steps))
	}
	if sc.BaseURL != "https://api.toko.co.id" {
		t.Errorf("expected base URL derived from first call, got %s", sc.BaseURL)
	}

	first := sc.Steps[sc.InitialStepID]
	if first.Request.Method != "GET" || first.Request.Path != "/v1/items" {
		t.Errorf("expected GET /v1/items first, got %s %s", first.Request.Method, first.Request.Path)
	}
}

func TestImportK6ScriptRejectsNoRecognizedCalls(t *testing.T) {
	if _, err := ImportK6Script(context.Background(), "export default function() { console.log('nothing here'); }", GeneratorConfig{}); err == nil {
		t.Error("expected error when no http.*() calls are found")
	}
}

func TestImportK6ScriptRejectsVariableOnlyURLs(t *testing.T) {
	script := `
export default function () {
  const base = __ENV.BASE_URL;
  http.get(base + '/items');
}
`
	if _, err := ImportK6Script(context.Background(), script, GeneratorConfig{}); err == nil {
		t.Error("expected error when URLs are built from variables rather than string literals")
	}
}
