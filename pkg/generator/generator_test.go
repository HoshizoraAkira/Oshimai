package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/oshimai/twin/pkg/vusession"
)

const sampleOpenAPIYAML = `
openapi: 3.0.3
info:
  title: Sample E-Commerce API
  version: 1.0.0
paths:
  /api/v1/auth/login:
    post:
      summary: Login user
      operationId: loginUser
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                username:
                  type: string
                  example: shopper_admin
                password:
                  type: string
                  format: password
      responses:
        '200':
          description: Login successful
          content:
            application/json:
              schema:
                type: object
                properties:
                  token:
                    type: string
  /api/v1/products:
    get:
      summary: List products
      operationId: listProducts
      responses:
        '200':
          description: Product list
    post:
      summary: Create product
      operationId: createProduct
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                price:
                  type: number
      responses:
        '201':
          description: Product created
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: integer
  /api/v1/products/{id}:
    get:
      summary: Get product details
      operationId: getProductDetails
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
      responses:
        '200':
          description: Product details
`

// TestOpenAPIParser verifies endpoint analysis, JWT extraction detection, and path variable normalization.
func TestOpenAPIParser(t *testing.T) {
	endpoints, doc, err := ParseOpenAPISpec(context.Background(), []byte(sampleOpenAPIYAML))
	if err != nil {
		t.Fatalf("failed to parse OpenAPI spec: %v", err)
	}

	if doc == nil || doc.Info.Title != "Sample E-Commerce API" {
		t.Fatalf("unexpected doc info: %+v", doc)
	}

	if len(endpoints) != 4 {
		t.Fatalf("expected 4 operations, got %d", len(endpoints))
	}

	// Verify login endpoint
	loginEP := endpoints[0]
	if !loginEP.ProducesAuthToken {
		t.Errorf("expected login endpoint to produce auth token")
	}
	if !strings.Contains(loginEP.DefaultPayload, "shopper_admin") {
		t.Errorf("expected payload to contain shopper_admin, got: %s", loginEP.DefaultPayload)
	}

	// Verify get product by ID endpoint normalization
	var getDetailsEP *EndpointDef
	for _, ep := range endpoints {
		if ep.ID == "getProductDetails" {
			getDetailsEP = ep
			break
		}
	}

	if getDetailsEP == nil {
		t.Fatalf("getProductDetails endpoint not found")
	}
	if getDetailsEP.NormalizedPath != "/api/v1/products/${id}" {
		t.Errorf("expected path to be normalized to '/api/v1/products/${id}', got %q", getDetailsEP.NormalizedPath)
	}
}

// TestOTelBehavioralMiner verifies chronological user journey reconstruction and transition matrix calculation.
func TestOTelBehavioralMiner(t *testing.T) {
	now := time.Now()

	spans := []OTelSpan{
		// Trace 1: Login -> List Products -> Create Product -> END
		{
			TraceID:   "trace_1",
			SpanID:    "s1",
			Method:    "POST",
			Route:     "/api/v1/auth/login",
			StartTime: now,
			EndTime:   now.Add(10 * time.Millisecond),
		},
		{
			TraceID:   "trace_1",
			SpanID:    "s2",
			Method:    "GET",
			Route:     "/api/v1/products",
			StartTime: now.Add(20 * time.Millisecond),
			EndTime:   now.Add(30 * time.Millisecond),
		},
		{
			TraceID:   "trace_1",
			SpanID:    "s3",
			Method:    "POST",
			Route:     "/api/v1/products",
			StartTime: now.Add(40 * time.Millisecond),
			EndTime:   now.Add(50 * time.Millisecond),
		},

		// Trace 2: Login -> List Products -> END (User dropped off after browsing)
		{
			TraceID:   "trace_2",
			SpanID:    "s4",
			Method:    "POST",
			Route:     "/api/v1/auth/login",
			StartTime: now.Add(100 * time.Millisecond),
			EndTime:   now.Add(110 * time.Millisecond),
		},
		{
			TraceID:   "trace_2",
			SpanID:    "s5",
			Method:    "GET",
			Route:     "/api/v1/products",
			StartTime: now.Add(120 * time.Millisecond),
			EndTime:   now.Add(130 * time.Millisecond),
		},
	}

	matrix, err := MineBehavioralTransitions(spans)
	if err != nil {
		t.Fatalf("failed to mine transitions: %v", err)
	}

	// Login -> List Products should be 100% (2 out of 2)
	loginProbs := matrix.Probabilities["POST /api/v1/auth/login"]
	if loginProbs["GET /api/v1/products"] != 1.0 {
		t.Errorf("expected 100%% transition from login to list products, got %f",
			loginProbs["GET /api/v1/products"])
	}

	// List Products -> Create Product (1 out of 2 = 50%), List Products -> END (1 out of 2 = 50%)
	listProbs := matrix.Probabilities["GET /api/v1/products"]
	if listProbs["POST /api/v1/products"] != 0.5 {
		t.Errorf("expected 50%% transition to Create Product, got %f", listProbs["POST /api/v1/products"])
	}
	if listProbs["END"] != 0.5 {
		t.Errorf("expected 50%% transition to END, got %f", listProbs["END"])
	}
}

// TestPayloadSynthesizer verifies mock JSON synthesis according to schema types and formats.
func TestPayloadSynthesizer(t *testing.T) {
	schema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: openapi3.Schemas{
			"email": &openapi3.SchemaRef{
				Value: &openapi3.Schema{Type: &openapi3.Types{"string"}, Format: "email"},
			},
			"account_id": &openapi3.SchemaRef{
				Value: &openapi3.Schema{Type: &openapi3.Types{"string"}, Format: "uuid"},
			},
			"active": &openapi3.SchemaRef{
				Value: &openapi3.Schema{Type: &openapi3.Types{"boolean"}},
			},
			"credits": &openapi3.SchemaRef{
				Value: &openapi3.Schema{Type: &openapi3.Types{"integer"}},
			},
			"tags": &openapi3.SchemaRef{
				Value: &openapi3.Schema{
					Type:  &openapi3.Types{"array"},
					Items: &openapi3.SchemaRef{Value: &openapi3.Schema{Type: &openapi3.Types{"string"}}},
				},
			},
		},
	}

	payload, err := SynthesizePayload(schema)
	if err != nil {
		t.Fatalf("failed to synthesize payload: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		t.Fatalf("synthesized payload is not valid JSON: %s (err: %v)", payload, err)
	}

	if parsed["email"] != "user@example.com" {
		t.Errorf("expected email 'user@example.com', got %v", parsed["email"])
	}
	if parsed["active"] != true {
		t.Errorf("expected active=true, got %v", parsed["active"])
	}
}

// TestEndToEndScenarioSynthesisAndExecution tests the entire pipeline:
// Ingest OpenAPI + OTel -> Synthesize Scenario -> Validate -> Execute against live httptest.Server!
func TestEndToEndScenarioSynthesisAndExecution(t *testing.T) {
	var loginHit, listHit int32
	var capturedAuthHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/login":
			atomic.AddInt32(&loginHit, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"token": "synthesized-jwt-999"}`))

		case "/api/v1/products":
			atomic.AddInt32(&listHit, 1)
			capturedAuthHeader = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id": 1, "name": "Test Item"}]`))

		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "ok"}`))
		}
	}))
	defer server.Close()

	// 1. Synthetic OTel Traces matching server
	otelLog := fmt.Sprintf(`[
		{"trace_id":"t1","name":"POST /api/v1/auth/login","method":"POST","route":"/api/v1/auth/login","start_time":"2026-09-03T10:00:00Z","end_time":"2026-09-03T10:00:01Z"},
		{"trace_id":"t1","name":"GET /api/v1/products","method":"GET","route":"/api/v1/products","start_time":"2026-09-03T10:00:02Z","end_time":"2026-09-03T10:00:03Z"}
	]`)

	// 2. Synthesize Scenario
	sc, err := Synthesize(context.Background(), []byte(sampleOpenAPIYAML), []byte(otelLog), GeneratorConfig{
		ScenarioID:   "e2e_synthesized_test",
		ScenarioName: "End-to-End Synthesized Scenario",
		BaseURL:      server.URL,
	})
	if err != nil {
		t.Fatalf("synthesis failed: %v", err)
	}

	// 3. Test Export YAML and JSON
	yamlBytes, err := ExportYAML(sc)
	if err != nil || len(yamlBytes) == 0 {
		t.Fatalf("failed to export YAML: %v", err)
	}

	jsonBytes, err := ExportJSON(sc)
	if err != nil || len(jsonBytes) == 0 {
		t.Fatalf("failed to export JSON: %v", err)
	}

	// 4. Verify initial step is login
	if sc.InitialStepID != "loginUser" {
		t.Errorf("expected InitialStepID to be loginUser, got %q", sc.InitialStepID)
	}

	// 5. Execute with VirtualUser Engine from Modul 1!
	vu, err := vusession.NewVirtualUser(vusession.Config{
		ID:       "synth-tester-01",
		Scenario: sc,
		Client:   server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create VirtualUser: %v", err)
	}

	report, err := vu.Execute(context.Background())
	if err != nil {
		t.Fatalf("scenario execution failed: %v", err)
	}

	if !report.Success {
		t.Fatalf("scenario execution reported failure: %s", report.FailureReason)
	}

	if atomic.LoadInt32(&loginHit) != 1 {
		t.Errorf("expected login to be called once, got %d", loginHit)
	}
	if atomic.LoadInt32(&listHit) != 1 {
		t.Errorf("expected list products to be called once, got %d", listHit)
	}

	// Ensure token was dynamically extracted from step 1 and injected into step 2 header!
	expectedBearer := "Bearer synthesized-jwt-999"
	if capturedAuthHeader != expectedBearer {
		t.Errorf("expected Authorization header %q, got %q", expectedBearer, capturedAuthHeader)
	}

	t.Logf("Successfully executed synthesized scenario with %d steps across %v",
		len(report.StepResults), report.TotalDuration)
}

// TestLLMEnricherFallback verifies fallback behavior of the pluggable LLM enricher.
func TestLLMEnricherFallback(t *testing.T) {
	enricher := NewLLMEnricher(nil) // nil client falls back cleanly to heuristic

	sc := &vusession.Scenario{
		ID:            "test_fallback",
		InitialStepID: "login",
		Steps: map[string]*vusession.Step{
			"login": {
				ID: "login",
				Request: vusession.RequestConfig{
					Path: "/login",
				},
			},
			"profile": {
				ID: "profile",
				Request: vusession.RequestConfig{
					Path: "/profile",
				},
			},
		},
	}

	enriched, err := enricher.Enrich(context.Background(), sc, nil)
	if err != nil {
		t.Fatalf("enrichment failed: %v", err)
	}

	// Profile step should have received Authorization header from enricher
	profileHdr := enriched.Steps["profile"].Request.Headers["Authorization"]
	if profileHdr != "Bearer ${jwt_token}" {
		t.Errorf("expected profile step to receive Bearer token header, got %q", profileHdr)
	}
}

// TestOTelSpanParsingFormats tests JSON array and NDJSON parsing.
func TestOTelSpanParsingFormats(t *testing.T) {
	ndjson := `{"trace_id":"t1","name":"GET /a","start_time":"2026-09-03T10:00:00Z","end_time":"2026-09-03T10:00:01Z"}
{"trace_id":"t1","name":"GET /b","start_time":"2026-09-03T10:00:02Z","end_time":"2026-09-03T10:00:03Z"}`

	spans, err := ParseOTelSpansJSON([]byte(ndjson))
	if err != nil {
		t.Fatalf("failed to parse NDJSON: %v", err)
	}
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
}

type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

// TestLLMClientEnricher verifies LLM client integration and error handling.
func TestLLMClientEnricher(t *testing.T) {
	// 1. Successful LLM completion
	clientSuccess := &mockLLMClient{response: `{"status":"optimized"}`}
	enricher := NewLLMEnricher(clientSuccess)

	sc := &vusession.Scenario{
		ID:            "llm_test",
		InitialStepID: "step_auth",
		Steps: map[string]*vusession.Step{
			"step_auth": {
				ID: "step_auth",
				Request: vusession.RequestConfig{
					Path: "/auth",
				},
			},
		},
	}

	enriched, err := enricher.Enrich(context.Background(), sc, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(enriched.Steps["step_auth"].Extractors) == 0 {
		t.Errorf("expected heuristic enricher to add extractors")
	}

	// 2. Failed LLM completion falls back gracefully
	clientFail := &mockLLMClient{err: fmt.Errorf("rate limit exceeded")}
	enricherFail := NewLLMEnricher(clientFail)
	_, err = enricherFail.Enrich(context.Background(), sc, nil)
	if err != nil {
		t.Fatalf("expected graceful fallback on LLM error, got error: %v", err)
	}
}

// TestSynthesizeWithoutOTelFallback verifies sequential synthesis when no OTel traces are provided.
func TestSynthesizeWithoutOTelFallback(t *testing.T) {
	sc, err := Synthesize(context.Background(), []byte(sampleOpenAPIYAML), nil, GeneratorConfig{
		ScenarioID: "seq_test",
		BaseURL:    "http://api.local",
	})
	if err != nil {
		t.Fatalf("failed to synthesize: %v", err)
	}

	if len(sc.Steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(sc.Steps))
	}

	// Validation check
	if err := vusession.ValidateScenario(sc); err != nil {
		t.Errorf("synthesized scenario failed validation: %v", err)
	}
}

// TestPayloadEdgeCases verifies various schema format handling.
func TestPayloadEdgeCases(t *testing.T) {
	schema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: openapi3.Schemas{
			"published_at": &openapi3.SchemaRef{
				Value: &openapi3.Schema{Type: &openapi3.Types{"string"}, Format: "date-time"},
			},
			"birth_date": &openapi3.SchemaRef{
				Value: &openapi3.Schema{Type: &openapi3.Types{"string"}, Format: "date"},
			},
			"website": &openapi3.SchemaRef{
				Value: &openapi3.Schema{Type: &openapi3.Types{"string"}, Format: "uri"},
			},
			"status": &openapi3.SchemaRef{
				Value: &openapi3.Schema{Type: &openapi3.Types{"string"}, Enum: []any{"active", "pending"}},
			},
			"price": &openapi3.SchemaRef{
				Value: &openapi3.Schema{Type: &openapi3.Types{"number"}},
			},
		},
	}

	jsonStr, err := SynthesizePayload(schema)
	if err != nil {
		t.Fatalf("failed to synthesize: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid json: %s", jsonStr)
	}

	if parsed["status"] != "active" {
		t.Errorf("expected enum 'active', got %v", parsed["status"])
	}
	if parsed["website"] != "https://example.com/resource" {
		t.Errorf("expected uri, got %v", parsed["website"])
	}
}
