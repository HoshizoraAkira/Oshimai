package generator

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

var pathVarRegex = regexp.MustCompile(`\{([^}]+)\}`)

// ParseOpenAPISpec parses an OpenAPI 3.x YAML/JSON document into EndpointDefs and dependency metadata.
func ParseOpenAPISpec(ctx context.Context, data []byte) ([]*EndpointDef, *openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false

	doc, err := loader.LoadFromData(data)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse OpenAPI v3 document: %w", err)
	}

	if err := doc.Validate(ctx); err != nil {
		// Log or tolerate minor validation warnings if paths exist
		if doc.Paths == nil || doc.Paths.Len() == 0 {
			return nil, nil, fmt.Errorf("OpenAPI document has no paths or invalid schema: %w", err)
		}
	}

	var endpoints []*EndpointDef

	// Iterate all paths
	pathsMap := doc.Paths.Map()
	pathKeys := make([]string, 0, len(pathsMap))
	for p := range pathsMap {
		pathKeys = append(pathKeys, p)
	}
	sort.Strings(pathKeys)

	for _, path := range pathKeys {
		pathItem := pathsMap[path]
		if pathItem == nil {
			continue
		}

		ops := pathItem.Operations()
		methodKeys := make([]string, 0, len(ops))
		for m := range ops {
			methodKeys = append(methodKeys, m)
		}
		sort.Strings(methodKeys)

		for _, method := range methodKeys {
			op := ops[method]
			if op == nil {
				continue
			}

			ep := analyzeOperation(path, method, op, doc)
			endpoints = append(endpoints, ep)
		}
	}

	return endpoints, doc, nil
}

func analyzeOperation(path, method string, op *openapi3.Operation, doc *openapi3.T) *EndpointDef {
	opID := op.OperationID
	if opID == "" {
		opID = fmt.Sprintf("%s_%s", strings.ToLower(method), sanitizePath(path))
	}

	summary := op.Summary
	if summary == "" {
		summary = fmt.Sprintf("%s %s", method, path)
	}

	// Transform path variables: /items/{id} -> /items/${id}
	normalizedPath := pathVarRegex.ReplaceAllString(path, "${$1}")

	ep := &EndpointDef{
		ID:             opID,
		Method:         strings.ToUpper(method),
		Path:           path,
		NormalizedPath: normalizedPath,
		OperationID:    opID,
		Summary:        summary,
		ProducedVars:   make([]string, 0),
		RequiredVars:   make([]string, 0),
		ExpectedStatus: http.StatusOK,
	}

	// 1. Check path parameters
	matches := pathVarRegex.FindAllStringSubmatch(path, -1)
	for _, m := range matches {
		if len(m) > 1 {
			ep.RequiredVars = append(ep.RequiredVars, m[1])
		}
	}

	// 2. Identify Auth login endpoints
	pathLower := strings.ToLower(path)
	if strings.Contains(pathLower, "login") || strings.Contains(pathLower, "auth") ||
		strings.Contains(pathLower, "token") || strings.Contains(pathLower, "signin") {
		ep.ProducesAuthToken = true
		ep.ProducedVars = append(ep.ProducedVars, "jwt_token")
	}

	// 3. Identify Auth requirement
	hasOpSec := op.Security != nil && len(*op.Security) > 0
	hasDocSec := doc.Security != nil && len(doc.Security) > 0
	if (hasOpSec || hasDocSec) && !ep.ProducesAuthToken {
		ep.RequiresAuth = true
	}

	// 4. Identify Resource Creation (POST returning ID)
	if ep.Method == http.MethodPost {
		ep.ExpectedStatus = http.StatusCreated
		resourceName := extractResourceBase(path)
		if resourceName != "" && !ep.ProducesAuthToken {
			idVar := fmt.Sprintf("%s_id", resourceName)
			ep.ProducedVars = append(ep.ProducedVars, idVar)
		}
	}

	// 5. Synthesize Request Body Payload if schema is present
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		content := op.RequestBody.Value.Content
		if jsonMedia, ok := content["application/json"]; ok && jsonMedia.Schema != nil && jsonMedia.Schema.Value != nil {
			payload, _ := SynthesizePayload(jsonMedia.Schema.Value)
			ep.DefaultPayload = payload
		}
	}

	return ep
}

func sanitizePath(path string) string {
	s := strings.Trim(path, "/")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

func extractResourceBase(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	if strings.HasPrefix(last, "{") {
		if len(parts) > 1 {
			last = parts[len(parts)-2]
		}
	}
	// singularize common plural suffixes
	last = strings.TrimSuffix(last, "s")
	return strings.ToLower(last)
}
