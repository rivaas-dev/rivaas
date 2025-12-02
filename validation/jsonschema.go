// Copyright 2025 The Rivaas Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// JSON Schema validation constants.
const (
	// defaultMaxCachedSchemas is the default maximum number of JSON schemas to cache.
	// Override with [WithMaxCachedSchemas].
	defaultMaxCachedSchemas = 1024

	// maxRecursionDepth limits recursion depth to prevent stack overflow from deeply nested structures.
	maxRecursionDepth = 100
)

// jsonschemaSchema is a type alias for the github.com/santhosh-tekuri/jsonschema/v6 Schema type.
type jsonschemaSchema = jsonschema.Schema

// validateWithSchema validates using JSON Schema ([StrategyJSONSchema]).
// The schema can be provided via [JSONSchemaProvider] interface or [WithCustomSchema] option.
func (v *Validator) validateWithSchema(ctx context.Context, val any, cfg *config) error {
	schemaID, schemaJSON := getSchemaForValue(val, cfg)
	if schemaJSON == "" {
		return nil
	}

	schema, err := v.getOrCompileSchema(schemaID, schemaJSON)
	if err != nil {
		return &Error{Fields: []FieldError{{Code: "schema_compile_error", Message: err.Error()}}}
	}

	var jsonBytes []byte

	// Use raw JSON from context if available and not partial
	if !cfg.partial {
		if rawJSON, ok := ExtractRawJSONCtx(ctx); ok && len(rawJSON) > 0 {
			jsonBytes = rawJSON
		}
	}

	// Otherwise marshal
	if jsonBytes == nil {
		var err error
		jsonBytes, err = json.Marshal(val)
		if err != nil {
			return &Error{Fields: []FieldError{{Code: "marshal_error", Message: err.Error()}}}
		}
	}

	// Decode and prune for partial mode
	var data any
	if cfg.partial && cfg.presence != nil {
		if err := json.Unmarshal(jsonBytes, &data); err != nil {
			return &Error{Fields: []FieldError{{Code: "unmarshal_error", Message: err.Error()}}}
		}

		data = pruneByPresence(data, "", cfg.presence, 0)
		// Preserve original bytes in case marshal fails
		originalBytes := jsonBytes
		prunedBytes, err := json.Marshal(data)
		if err != nil {
			// If marshal fails, fall back to original jsonBytes
			jsonBytes = originalBytes
		} else {
			jsonBytes = prunedBytes
		}
	}

	// Unmarshal data for validation
	if data == nil {
		if err := json.Unmarshal(jsonBytes, &data); err != nil {
			return &Error{Fields: []FieldError{{Code: "unmarshal_error", Message: err.Error()}}}
		}
	}

	// Validate
	if err := schema.Validate(data); err != nil {
		if verr, ok := err.(*jsonschema.ValidationError); ok {
			return formatSchemaErrors(verr, cfg)
		}
		return &Error{Fields: []FieldError{{Code: "schema_validation_error", Message: err.Error()}}}
	}

	return nil
}

// pruneByPresence removes non-present fields from JSON data for partial validation.
// It uses nil placeholders for arrays to maintain array length.
// The depth parameter tracks recursion depth to prevent stack overflow (max: [maxRecursionDepth]).
func pruneByPresence(data any, prefix string, pm PresenceMap, depth int) any {
	if depth > maxRecursionDepth {
		return data // Prevent stack overflow from deeply nested structures
	}

	switch t := data.(type) {
	case map[string]any:
		out := make(map[string]any)
		for k, v := range t {
			//nolint:copyloopvar // path is modified conditionally
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}

			if pm.HasPrefix(path) {
				out[k] = pruneByPresence(v, path, pm, depth+1)
			}
		}
		return out

	case []any:
		// Keep array length with nil placeholders
		out := make([]any, len(t))
		for i, v := range t {
			path := prefix + "." + strconv.Itoa(i)
			if pm.HasPrefix(path) {
				out[i] = pruneByPresence(v, path, pm, depth+1)
			} else {
				out[i] = nil
			}
		}
		return out

	default:
		return t
	}
}

// getSchemaForValue retrieves JSON Schema for a value.
func getSchemaForValue(v any, cfg *config) (id, schema string) {
	if cfg.customSchema != "" {
		return cfg.customSchemaID, cfg.customSchema
	}

	if provider, ok := v.(JSONSchemaProvider); ok {
		return provider.JSONSchema()
	}

	return "", ""
}

// compileSchema compiles a JSON Schema from a JSON string.
func compileSchema(id, schemaJSON string) (*jsonschemaSchema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()  // Enable format validation
	compiler.AssertContent() // Enable content validation

	// Parse schema JSON
	var schemaDoc any
	if err := json.Unmarshal([]byte(schemaJSON), &schemaDoc); err != nil {
		return nil, fmt.Errorf("invalid schema JSON: %w", err)
	}

	// Add schema resource
	schemaURL := id
	if schemaURL == "" {
		schemaURL = "schema.json"
	}
	if err := compiler.AddResource(schemaURL, schemaDoc); err != nil {
		return nil, fmt.Errorf("failed to add schema resource: %w", err)
	}

	// Compile schema
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	return schema, nil
}

// formatSchemaErrors formats JSON Schema errors into an [*Error] with stable codes.
// formatSchemaErrors flattens the structured ValidationError tree into [FieldError] values.
func formatSchemaErrors(verr *jsonschema.ValidationError, cfg *config) error {
	var result Error

	// Recursively collect all validation errors
	collectSchemaErrors(verr, &result, cfg)

	result.Sort()
	return &result
}

// collectSchemaErrors recursively collects validation errors from the error tree into [*Error].
func collectSchemaErrors(verr *jsonschema.ValidationError, result *Error, cfg *config) {
	if verr == nil {
		return
	}

	// Build field path from instance location
	field := strings.Join(verr.InstanceLocation, ".")
	field = strings.TrimPrefix(field, ".")

	if cfg.fieldNameMapper != nil && field != "" {
		field = cfg.fieldNameMapper(field)
	}

	// Extract error kind as code
	// ErrorKind is an interface in v6, use fmt.Sprintf to get string representation
	errorKind := fmt.Sprintf("%v", verr.ErrorKind)
	code := "schema." + errorKind

	// Get error message
	message := verr.Error()

	// Add error if it has a meaningful message (leaf error)
	if len(verr.Causes) == 0 {
		result.Add(field, code, message, map[string]any{
			"kind":       errorKind,
			"schema_url": verr.SchemaURL,
		})

		if cfg.maxErrors > 0 && len(result.Fields) >= cfg.maxErrors {
			result.Truncated = true
			return
		}
	}

	// Recursively process nested errors
	for _, cause := range verr.Causes {
		if cfg.maxErrors > 0 && len(result.Fields) >= cfg.maxErrors {
			result.Truncated = true
			return
		}
		collectSchemaErrors(cause, result, cfg)
	}
}
