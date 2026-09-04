package vusession

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var (
	regexCacheMu sync.RWMutex
	regexCache   = make(map[string]*regexp.Regexp)
)

func getCompiledRegex(pattern string) (*regexp.Regexp, error) {
	regexCacheMu.RLock()
	re, ok := regexCache[pattern]
	regexCacheMu.RUnlock()
	if ok {
		return re, nil
	}

	regexCacheMu.Lock()
	defer regexCacheMu.Unlock()
	if re, ok = regexCache[pattern]; ok {
		return re, nil
	}

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	regexCache[pattern] = compiled
	return compiled, nil
}

// Extract processes an ExtractorConfig against an HTTP response and raw body.
func Extract(cfg ExtractorConfig, resp *http.Response, body []byte) (any, error) {
	if cfg.TargetVar == "" {
		return nil, fmt.Errorf("target_var must not be empty")
	}

	var extracted any
	var err error

	switch cfg.Source {
	case ExtractorSourceStatusCode:
		if resp != nil {
			extracted = resp.StatusCode
		}

	case ExtractorSourceHeader:
		if resp != nil {
			hdrVal := resp.Header.Get(cfg.Path)
			if hdrVal != "" {
				extracted = hdrVal
			}
		}

	case ExtractorSourceBodyJSON:
		extracted, err = extractJSONPath(body, cfg.Path)

	case ExtractorSourceRegex:
		extracted, err = extractRegex(body, cfg.Regex)

	default:
		return nil, fmt.Errorf("unknown extractor source: %q", cfg.Source)
	}

	if err != nil || extracted == nil {
		if cfg.Default != nil {
			return cfg.Default, nil
		}
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("extraction source %s at %q yielded no value", cfg.Source, cfg.Path)
	}

	return extracted, nil
}

// extractJSONPath parses raw JSON body and traverses dot-separated keys and array indices (e.g. "data.users.0.token").
func extractJSONPath(body []byte, path string) (any, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("response body is empty, cannot extract json")
	}

	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response body: %w", err)
	}

	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" || trimmedPath == "." {
		return parsed, nil
	}

	tokens := strings.Split(trimmedPath, ".")
	var current any = parsed

	for _, token := range tokens {
		if token == "" {
			continue
		}

		switch node := current.(type) {
		case map[string]any:
			val, ok := node[token]
			if !ok {
				return nil, fmt.Errorf("key %q not found in JSON object", token)
			}
			current = val

		case []any:
			idx, err := strconv.Atoi(token)
			if err != nil {
				return nil, fmt.Errorf("expected array index in %q, got non-integer %q", path, token)
			}
			if idx < 0 || idx >= len(node) {
				return nil, fmt.Errorf("index %d out of bounds (len %d)", idx, len(node))
			}
			current = node[idx]

		default:
			return nil, fmt.Errorf("cannot traverse key %q on primitive type %T", token, current)
		}
	}

	return current, nil
}

// extractRegex executes regex on body and returns first capture group, or entire match if no capture group.
func extractRegex(body []byte, pattern string) (string, error) {
	if pattern == "" {
		return "", fmt.Errorf("regex pattern cannot be empty")
	}

	re, err := getCompiledRegex(pattern)
	if err != nil {
		return "", err
	}

	matches := re.FindSubmatch(body)
	if len(matches) == 0 {
		return "", fmt.Errorf("pattern %q did not match body", pattern)
	}

	if len(matches) > 1 {
		// Return first capture group
		return string(matches[1]), nil
	}
	// Return full match
	return string(matches[0]), nil
}
