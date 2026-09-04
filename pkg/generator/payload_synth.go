package generator

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// SynthesizePayload generates realistic JSON mock data for an OpenAPI request body schema.
func SynthesizePayload(schema *openapi3.Schema) (string, error) {
	if schema == nil {
		return "", nil
	}

	val := generateValueForSchema(schema, "")
	if val == nil {
		return "", nil
	}

	bytes, err := json.MarshalIndent(val, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func generateValueForSchema(schema *openapi3.Schema, fieldName string) any {
	if schema == nil {
		return nil
	}

	// 1. Example / Default value takes priority
	if schema.Default != nil {
		return schema.Default
	}
	if schema.Example != nil {
		return schema.Example
	}

	// 2. Enums
	if len(schema.Enum) > 0 {
		return schema.Enum[0]
	}

	// Determine type
	typeName := ""
	if schema.Type != nil && len(*schema.Type) > 0 {
		typeName = (*schema.Type)[0]
	}

	switch typeName {
	case "string":
		return synthesizeString(schema, fieldName)

	case "integer":
		if schema.Min != nil {
			return int(*schema.Min)
		}
		if strings.Contains(strings.ToLower(fieldName), "quantity") || strings.Contains(strings.ToLower(fieldName), "count") {
			return 2
		}
		return 1

	case "number":
		if schema.Min != nil {
			return *schema.Min
		}
		if strings.Contains(strings.ToLower(fieldName), "price") || strings.Contains(strings.ToLower(fieldName), "amount") {
			return 99.50
		}
		return 10.0

	case "boolean":
		return true

	case "array":
		if schema.Items != nil && schema.Items.Value != nil {
			itemVal := generateValueForSchema(schema.Items.Value, fieldName)
			if itemVal != nil {
				return []any{itemVal}
			}
		}
		return []any{}

	case "object", "":
		if len(schema.Properties) > 0 {
			res := make(map[string]any)
			// Sort keys for deterministic output
			keys := make([]string, 0, len(schema.Properties))
			for k := range schema.Properties {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				propRef := schema.Properties[k]
				if propRef != nil && propRef.Value != nil {
					res[k] = generateValueForSchema(propRef.Value, k)
				}
			}
			return res
		}
	}

	return nil
}

func synthesizeString(schema *openapi3.Schema, fieldName string) string {
	fName := strings.ToLower(fieldName)

	// Check Format
	switch strings.ToLower(schema.Format) {
	case "email":
		return "user@example.com"
	case "uuid":
		return "e7b2b73e-3240-4c3e-8c3b-577884d5df23"
	case "date-time":
		return "2026-09-03T10:00:00Z"
	case "date":
		return "2026-09-03"
	case "uri", "url":
		return "https://example.com/resource"
	case "password":
		return "SecurePass123!"
	}

	// Heuristic field name matching
	if strings.Contains(fName, "email") {
		return "user@example.com"
	}
	if strings.Contains(fName, "password") || strings.Contains(fName, "secret") {
		return "SecurePass123!"
	}
	if strings.Contains(fName, "username") || strings.Contains(fName, "user") {
		return "shopper_01"
	}
	if strings.Contains(fName, "name") {
		return "Standard Item"
	}
	if strings.Contains(fName, "token") {
		return "sample-token-abc"
	}
	if strings.Contains(fName, "id") {
		return "id-1001"
	}

	return "test_val"
}
