package vusession

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
)

// RandomSource provides pseudo-random values for Markov transition branch selection.
type RandomSource interface {
	Float64() float64
}

// DefaultRandSource uses Go's math/rand/v2 standard library PCG generator.
type DefaultRandSource struct {
	mu  sync.Mutex
	pcg *rand.PCG
	r   *rand.Rand
}

func NewDefaultRandSource(seed uint64) *DefaultRandSource {
	pcg := rand.NewPCG(seed, seed^0x5851f42d4c957f2d)
	return &DefaultRandSource{
		pcg: pcg,
		r:   rand.New(pcg),
	}
}

func (s *DefaultRandSource) Float64() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.r.Float64()
}

// TransitionEvaluator selects the next Step based on transition conditions and Markov probability weights.
type TransitionEvaluator struct {
	randSrc RandomSource
}

// NewTransitionEvaluator creates a TransitionEvaluator with given random source.
func NewTransitionEvaluator(randSrc RandomSource) *TransitionEvaluator {
	if randSrc == nil {
		randSrc = NewDefaultRandSource(uint64(rand.Uint64()))
	}
	return &TransitionEvaluator{
		randSrc: randSrc,
	}
}

// SelectNextTransition filters transitions by Condition, then samples via Cumulative Distribution Function (CDF).
// Returns the selected Transition, or nil if no transitions or all conditions fail.
func (te *TransitionEvaluator) SelectNextTransition(step *Step, result *StepResult, state *SessionState) (*Transition, error) {
	if step == nil || len(step.Transitions) == 0 {
		return nil, nil
	}

	// 1. Filter transitions matching condition
	type candidate struct {
		transition *Transition
		weight     float64
	}

	var candidates []candidate
	var totalWeight float64

	for i := range step.Transitions {
		t := &step.Transitions[i]
		if te.evaluateCondition(t.Condition, result, state) {
			w := te.computeWeight(t)
			candidates = append(candidates, candidate{
				transition: t,
				weight:     w,
			})
			totalWeight += w
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	if len(candidates) == 1 || totalWeight <= 0 {
		return candidates[0].transition, nil
	}

	// 2. Sample transition using CDF
	roll := te.randSrc.Float64() * totalWeight
	var cumulative float64

	for _, c := range candidates {
		cumulative += c.weight
		if roll < cumulative {
			return c.transition, nil
		}
	}

	// Fallback to last candidate if floating point rounding slightly exceeds totalWeight
	return candidates[len(candidates)-1].transition, nil
}

func (te *TransitionEvaluator) computeWeight(t *Transition) float64 {
	if t.Weight > 0 {
		return float64(t.Weight)
	}
	if t.Probability > 0 {
		return t.Probability
	}
	return 1.0
}

// evaluateCondition checks condition strings against the current StepResult and SessionState.
// Supported syntax examples:
// - "" or empty (always true)
// - "status == 200", "status != 200", "status >= 400", "status < 300"
// - "status_code == 200"
// - "success == true", "success == false"
// - "state.key == value"
func (te *TransitionEvaluator) evaluateCondition(expr string, result *StepResult, state *SessionState) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true
	}

	if result == nil {
		return false
	}

	// Operators to check
	operators := []string{"==", "!=", ">=", "<=", ">", "<"}
	for _, op := range operators {
		idx := strings.Index(expr, op)
		if idx != -1 {
			left := strings.TrimSpace(expr[:idx])
			right := strings.TrimSpace(expr[idx+len(op):])
			return te.compareOperands(left, op, right, result, state)
		}
	}

	// Boolean flag expressions
	switch expr {
	case "success":
		return result.Success
	case "!success":
		return !result.Success
	default:
		return true
	}
}

func (te *TransitionEvaluator) compareOperands(left, op, right string, result *StepResult, state *SessionState) bool {
	// Resolve left value
	leftVal, leftIsNum, leftNum := te.resolveValue(left, result, state)
	// Resolve right value
	rightClean := strings.Trim(right, `"'`)
	rightNum, rightErr := strconv.ParseFloat(rightClean, 64)

	if leftIsNum && rightErr == nil {
		switch op {
		case "==":
			return leftNum == rightNum
		case "!=":
			return leftNum != rightNum
		case ">":
			return leftNum > rightNum
		case ">=":
			return leftNum >= rightNum
		case "<":
			return leftNum < rightNum
		case "<=":
			return leftNum <= rightNum
		}
	}

	// String comparison
	switch op {
	case "==":
		return leftVal == rightClean
	case "!=":
		return leftVal != rightClean
	default:
		return false
	}
}

func (te *TransitionEvaluator) resolveValue(target string, result *StepResult, state *SessionState) (strVal string, isNum bool, numVal float64) {
	targetLower := strings.ToLower(target)
	switch targetLower {
	case "status", "status_code":
		return strconv.Itoa(result.StatusCode), true, float64(result.StatusCode)
	case "success":
		return strconv.FormatBool(result.Success), false, 0
	}

	if strings.HasPrefix(target, "state.") {
		key := strings.TrimPrefix(target, "state.")
		if state != nil {
			if v, ok := state.Get(key); ok && v != nil {
				s := fmt.Sprintf("%v", v)
				if fn, err := strconv.ParseFloat(s, 64); err == nil {
					return s, true, fn
				}
				return s, false, 0
			}
		}
		return "", false, 0
	}

	return target, false, 0
}
