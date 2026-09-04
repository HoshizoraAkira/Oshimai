package generator

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/oshimai/twin/pkg/vusession"
)

// FuzzVariant is one malformed-input mutation applied to a request body for resilience smoke
// testing — the goal is not deep security fuzzing, but catching the cheapest, most common crash:
// an endpoint that 500s on a missing field, a wrong type, an oversized string, or unexpected
// unicode, instead of rejecting it gracefully with a 4xx.
type FuzzVariant struct {
	Name string
	Body string
}

const hugeStringFuzzLen = 200_000

// FuzzVariants derives schema-agnostic malformed variants of a JSON object payload by mutating
// one field at a time. It works purely from the JSON shape (no OpenAPI schema required), so it
// applies equally to payloads synthesized from OpenAPI, imported from HAR/Postman, or handwritten.
func FuzzVariants(payloadJSON string) []FuzzVariant {
	var obj map[string]any
	if err := json.Unmarshal([]byte(payloadJSON), &obj); err != nil || len(obj) == 0 {
		return nil
	}

	var firstKey string
	for k := range obj {
		firstKey = k
		break
	}

	variants := make([]FuzzVariant, 0, 5)

	if missing := mutateMissingField(obj, firstKey); missing != "" {
		variants = append(variants, FuzzVariant{Name: "missing_field", Body: missing})
	}
	if typeMismatch := mutateTypeMismatch(obj, firstKey); typeMismatch != "" {
		variants = append(variants, FuzzVariant{Name: "type_mismatch", Body: typeMismatch})
	}
	if huge := mutateValue(obj, firstKey, hugeString()); huge != "" {
		variants = append(variants, FuzzVariant{Name: "oversized_string", Body: huge})
	}
	if unicode := mutateValue(obj, firstKey, "\u202Eevil \U0001F525"); unicode != "" {
		variants = append(variants, FuzzVariant{Name: "unicode_control_chars", Body: unicode})
	}
	variants = append(variants, FuzzVariant{Name: "empty_object", Body: "{}"})

	return variants
}

func mutateMissingField(obj map[string]any, drop string) string {
	if drop == "" {
		return ""
	}
	clone := make(map[string]any, len(obj))
	for k, v := range obj {
		if k == drop {
			continue
		}
		clone[k] = v
	}
	b, err := json.Marshal(clone)
	if err != nil {
		return ""
	}
	return string(b)
}

func mutateTypeMismatch(obj map[string]any, key string) string {
	if key == "" {
		return ""
	}
	clone := make(map[string]any, len(obj))
	for k, v := range obj {
		clone[k] = v
	}
	switch clone[key].(type) {
	case string:
		clone[key] = 1234567890
	case float64:
		clone[key] = "not_a_number"
	case bool:
		clone[key] = "maybe"
	default:
		clone[key] = []any{"unexpected", "array"}
	}
	b, err := json.Marshal(clone)
	if err != nil {
		return ""
	}
	return string(b)
}

func mutateValue(obj map[string]any, key string, newVal any) string {
	if key == "" {
		return ""
	}
	clone := make(map[string]any, len(obj))
	for k, v := range obj {
		clone[k] = v
	}
	clone[key] = newVal
	b, err := json.Marshal(clone)
	if err != nil {
		return ""
	}
	return string(b)
}

func hugeString() string {
	b := make([]byte, hugeStringFuzzLen)
	for i := range b {
		b[i] = 'A'
	}
	return string(b)
}

// BuildFuzzScenario derives a standalone resilience-smoke-test Scenario from an already-synthesized
// base Scenario: every step with a non-empty POST/PUT/PATCH body is cloned once per fuzz variant,
// chained sequentially, and asserted to NOT return a 5xx (100-499 is accepted — anything from a
// clean 2xx to a graceful 4xx validation error counts as a pass; a 500-599 counts as the endpoint
// crashing on malformed input).
func BuildFuzzScenario(base *vusession.Scenario) (*vusession.Scenario, error) {
	if base == nil {
		return nil, fmt.Errorf("base scenario cannot be nil")
	}

	fuzzSteps := make(map[string]*vusession.Step)
	var order []string

	for _, stepID := range sortedStepIDs(base) {
		step := base.Steps[stepID]
		if step.Request.Body == "" {
			continue
		}
		method := step.Request.Method
		if method != "POST" && method != "PUT" && method != "PATCH" {
			continue
		}

		for _, variant := range FuzzVariants(step.Request.Body) {
			fuzzID := fmt.Sprintf("fuzz_%s_%s", stepID, variant.Name)
			fuzzSteps[fuzzID] = &vusession.Step{
				ID:   fuzzID,
				Name: fmt.Sprintf("Fuzz: %s (%s)", step.Name, variant.Name),
				Request: vusession.RequestConfig{
					Method:  method,
					Path:    step.Request.Path,
					Headers: step.Request.Headers,
					Body:    variant.Body,
					Timeout: step.Request.Timeout,
				},
				Assertions: []vusession.AssertionConfig{
					{Type: vusession.AssertStatusCodeInRange, MinCode: 100, MaxCode: 499},
				},
				OnFailure: vusession.FailurePolicy{Action: vusession.FailureActionAbort},
			}
			order = append(order, fuzzID)
		}
	}

	if len(fuzzSteps) == 0 {
		return nil, fmt.Errorf("no POST/PUT/PATCH steps with a request body found to fuzz")
	}

	// Chain sequentially so a single VU walks every fuzz variant once, then terminates.
	for i, id := range order {
		if i+1 < len(order) {
			fuzzSteps[id].Transitions = []vusession.Transition{{TargetStepID: order[i+1], Probability: 1.0}}
		} else {
			fuzzSteps[id].Transitions = []vusession.Transition{{TargetStepID: "END", Probability: 1.0}}
		}
	}

	fuzzScenario := &vusession.Scenario{
		ID:             base.ID + "_fuzz_smoke",
		Name:           base.Name + " — Resilience Fuzz Smoke Test",
		Description:    "Auto-generated malformed-payload smoke test: expects graceful 4xx rejection, flags any 5xx crash.",
		BaseURL:        base.BaseURL,
		InitialStepID:  order[0],
		DefaultHeaders: base.DefaultHeaders,
		Timeout:        base.Timeout,
		Steps:          fuzzSteps,
	}

	if err := vusession.ValidateScenario(fuzzScenario); err != nil {
		return nil, fmt.Errorf("generated fuzz scenario failed validation: %w", err)
	}
	return fuzzScenario, nil
}

func sortedStepIDs(sc *vusession.Scenario) []string {
	ids := make([]string, 0, len(sc.Steps))
	for id := range sc.Steps {
		ids = append(ids, id)
	}
	// Deterministic ordering keeps generated fuzz scenarios stable across runs.
	sort.Strings(ids)
	return ids
}
