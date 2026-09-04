package vusession

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseScenarioJSON parses raw JSON bytes into a validated Scenario.
func ParseScenarioJSON(data []byte) (*Scenario, error) {
	var sc Scenario
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("failed to parse scenario JSON: %w", err)
	}
	if err := ValidateScenario(&sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

// ParseScenarioYAML parses raw YAML bytes into a validated Scenario.
func ParseScenarioYAML(data []byte) (*Scenario, error) {
	var sc Scenario
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return nil, fmt.Errorf("failed to parse scenario YAML: %w", err)
	}
	if err := ValidateScenario(&sc); err != nil {
		return nil, err
	}
	return &sc, nil
}

// LoadScenarioFromFile reads and parses a Scenario from a file (.json, .yaml, or .yml).
func LoadScenarioFromFile(path string) (*Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read scenario file %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return ParseScenarioYAML(data)
	case ".json":
		return ParseScenarioJSON(data)
	default:
		// Attempt YAML first as YAML is a superset of JSON
		if sc, err := ParseScenarioYAML(data); err == nil {
			return sc, nil
		}
		return ParseScenarioJSON(data)
	}
}

// ValidateScenario verifies the semantic integrity and graph topology of a Scenario.
func ValidateScenario(sc *Scenario) error {
	if sc == nil {
		return fmt.Errorf("scenario cannot be nil")
	}
	if strings.TrimSpace(sc.ID) == "" {
		return fmt.Errorf("scenario id must not be empty")
	}
	if strings.TrimSpace(sc.InitialStepID) == "" {
		return fmt.Errorf("scenario initial_step_id must not be empty")
	}
	if len(sc.Steps) == 0 {
		return fmt.Errorf("scenario must contain at least one step")
	}

	if _, ok := sc.Steps[sc.InitialStepID]; !ok {
		return fmt.Errorf("initial_step_id %q is not defined in steps", sc.InitialStepID)
	}

	for stepID, step := range sc.Steps {
		if step == nil {
			return fmt.Errorf("step %q is nil", stepID)
		}
		if step.ID == "" {
			step.ID = stepID
		}

		// Validate transitions
		for i, tr := range step.Transitions {
			tgt := strings.TrimSpace(tr.TargetStepID)
			if tgt != "" && strings.ToUpper(tgt) != "END" {
				if _, ok := sc.Steps[tgt]; !ok {
					return fmt.Errorf("step %q transition[%d] points to non-existent step %q", stepID, i, tgt)
				}
			}
			if tr.Probability < 0 {
				return fmt.Errorf("step %q transition[%d] has negative probability: %f", stepID, i, tr.Probability)
			}
			if tr.Weight < 0 {
				return fmt.Errorf("step %q transition[%d] has negative weight: %d", stepID, i, tr.Weight)
			}
		}

		// Validate fallback step if transition policy configured
		if step.OnFailure.Action == FailureActionTransition {
			fb := step.OnFailure.FallbackStepID
			if fb == "" {
				return fmt.Errorf("step %q configured with failure action 'transition' but no fallback_step_id provided", stepID)
			}
			if _, ok := sc.Steps[fb]; !ok {
				return fmt.Errorf("step %q fallback_step_id %q does not exist", stepID, fb)
			}
		}
	}

	return nil
}
