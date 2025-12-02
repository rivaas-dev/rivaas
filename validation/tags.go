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
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

// validateWithTags validates using go-playground/validator struct tags ([StrategyTags]).
// It supports both full and partial validation modes.
func (v *Validator) validateWithTags(val any, cfg *config) error {
	v.initTagValidator()

	rv := reflect.ValueOf(val)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil
	}

	// Partial mode: validate only leaf fields that are present
	if cfg.partial && cfg.presence != nil {
		return v.validatePartialLeafsOnly(val, cfg)
	}

	// Full validation
	err := v.tagValidator.Struct(val)
	if err == nil {
		return nil
	}

	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		return v.formatTagErrors(validationErrs, val, cfg)
	}

	return &Error{Fields: []FieldError{{Code: "tag_error", Message: err.Error()}}}
}

// validatePartialLeafsOnly validates only leaf fields present in the [PresenceMap].
// It avoids enforcing "required" on nested fields that weren't provided,
// making it suitable for PATCH request validation.
func (v *Validator) validatePartialLeafsOnly(val any, cfg *config) error {
	leaves := cfg.presence.LeafPaths()
	if len(leaves) == 0 {
		return nil
	}

	// Sanity cap to prevent pathological inputs
	maxLeaves := 10000 // default
	if cfg.maxFields > 0 {
		maxLeaves = cfg.maxFields
	}
	if len(leaves) > maxLeaves {
		leaves = leaves[:maxLeaves]
	}

	var result Error
	structType := reflect.TypeOf(val)
	for structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	for _, path := range leaves {
		// Resolve field value and struct field
		fieldVal, structField, ok := v.resolvePath(val, path)
		if !ok {
			continue
		}

		// Get validation tag
		validateTag := structField.Tag.Get("validate")
		if validateTag == "" {
			continue
		}

		// Validate this single field
		if err := v.tagValidator.Var(fieldVal.Interface(), validateTag); err != nil {
			if verrs, ok := err.(validator.ValidationErrors); ok {
				// Format with proper path context
				for _, e := range verrs {
					code := "tag." + e.Tag()
					msg := getTagErrorMessage(e)

					// Redact if needed
					value := fmt.Sprint(e.Value())
					if cfg.redactor != nil && cfg.redactor(path) {
						value = "***REDACTED***"
						msg = strings.ReplaceAll(msg, fmt.Sprint(e.Value()), "***REDACTED***")
					}

					result.Add(path, code, msg, map[string]any{
						"tag":   e.Tag(),
						"param": e.Param(),
						"value": value,
					})
				}
			} else {
				result.AddError(err)
			}
		}

		// Check max errors
		if cfg.maxErrors > 0 && len(result.Fields) >= cfg.maxErrors {
			result.Truncated = true
			break
		}
	}

	if result.HasErrors() {
		result.Sort()
		return &result
	}
	return nil
}

// resolvePath resolves a dot-path (e.g., "items.2.name") to its reflect.Value and StructField.
// It returns (zero, zero, false) if the path cannot be resolved.
func (v *Validator) resolvePath(val any, path string) (reflect.Value, reflect.StructField, bool) {
	parts := strings.Split(path, ".")
	currentVal := reflect.ValueOf(val)
	var currentField reflect.StructField

	for i, part := range parts {
		// Dereference pointers
		for currentVal.Kind() == reflect.Ptr {
			if currentVal.IsNil() {
				return reflect.Value{}, reflect.StructField{}, false
			}
			currentVal = currentVal.Elem()
		}

		// Handle array/slice index
		if idx, err := strconv.Atoi(part); err == nil {
			if currentVal.Kind() == reflect.Slice || currentVal.Kind() == reflect.Array {
				if idx >= 0 && idx < currentVal.Len() {
					currentVal = currentVal.Index(idx)
					continue
				}
			}
			return reflect.Value{}, reflect.StructField{}, false
		}

		// Handle struct field
		if currentVal.Kind() == reflect.Struct {
			structType := currentVal.Type()
			fieldMap := v.getFieldMap(structType)

			if fieldIndex, found := fieldMap[part]; found {
				currentField = structType.Field(fieldIndex)
				currentVal = currentVal.Field(fieldIndex)
			} else {
				return reflect.Value{}, reflect.StructField{}, false
			}

			// If this is the last part, return
			if i == len(parts)-1 {
				return currentVal, currentField, true
			}

			continue
		}

		return reflect.Value{}, reflect.StructField{}, false
	}

	return currentVal, currentField, true
}

// getJSONFieldName extracts the JSON field name from a struct field tag.
func getJSONFieldName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")
	if jsonTag == "" || jsonTag == "-" {
		return field.Name
	}
	if idx := strings.Index(jsonTag, ","); idx != -1 {
		return jsonTag[:idx]
	}
	return jsonTag
}

// buildFieldMap builds a map of JSON field names to field indices for a struct type.
func buildFieldMap(structType reflect.Type) map[string]int {
	fieldMap := make(map[string]int, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		jsonName := getJSONFieldName(field)
		if jsonName != "" && jsonName != "-" {
			fieldMap[jsonName] = i
		}
	}
	return fieldMap
}

// formatTagErrors formats go-playground/validator errors into an [*Error] with stable codes.
func (v *Validator) formatTagErrors(errs validator.ValidationErrors, structValue any, cfg *config) error {
	var result Error
	structType := reflect.TypeOf(structValue)
	for structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	for _, e := range errs {
		ns := e.Namespace()
		structNS := e.StructNamespace()

		// Strip top struct name
		if idx := strings.Index(structNS, "."); idx != -1 {
			ns = ns[idx+1:]
		}

		path := v.getCachedJSONPath(ns, structType)

		if cfg.fieldNameMapper != nil {
			path = cfg.fieldNameMapper(path)
		}

		// Stable code
		code := "tag." + e.Tag()
		msg := getTagErrorMessage(e)

		// Redact
		value := fmt.Sprint(e.Value())
		if cfg.redactor != nil && cfg.redactor(path) {
			value = "***REDACTED***"
			msg = strings.ReplaceAll(msg, fmt.Sprint(e.Value()), "***REDACTED***")
		}

		result.Add(path, code, msg, map[string]any{
			"tag":   e.Tag(),
			"param": e.Param(),
			"value": value,
		})

		if cfg.maxErrors > 0 && len(result.Fields) >= cfg.maxErrors {
			result.Truncated = true
			break
		}
	}

	result.Sort()
	return &result
}

// namespaceToJSONPath converts validator namespace to JSON path using struct tags.
func namespaceToJSONPath(ns string, structType reflect.Type) string {
	parts := strings.Split(ns, ".")
	result := make([]string, 0, len(parts))

	currentType := structType
	for _, part := range parts {
		// Numeric index
		if idx, err := strconv.Atoi(part); err == nil {
			result = append(result, strconv.Itoa(idx))
			if currentType.Kind() == reflect.Slice || currentType.Kind() == reflect.Array {
				currentType = currentType.Elem()
			}
			continue
		}

		// Struct field
		if currentType.Kind() == reflect.Struct {
			if field, found := currentType.FieldByName(part); found {
				jsonName := getJSONFieldName(field)
				result = append(result, jsonName)
				currentType = field.Type

				if currentType.Kind() == reflect.Ptr {
					currentType = currentType.Elem()
				}
				continue
			}
		}

		// Fallback
		result = append(result, strings.ToLower(part))
	}

	return strings.Join(result, ".")
}

// getTagErrorMessage returns a human-readable error message for a tag error.
func getTagErrorMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "url":
		return "must be a valid URL"
	case "min":
		if e.Type().Kind() == reflect.String {
			return fmt.Sprintf("must be at least %s characters", e.Param())
		}
		return fmt.Sprintf("must be at least %s", e.Param())
	case "max":
		if e.Type().Kind() == reflect.String {
			return fmt.Sprintf("must be at most %s characters", e.Param())
		}
		return fmt.Sprintf("must be at most %s", e.Param())
	case "oneof":
		return fmt.Sprintf("must be one of [%s]", e.Param())
	case "username":
		return "must be 3-20 alphanumeric characters or underscore"
	case "slug":
		return "must be lowercase letters, numbers, and hyphens"
	case "strong_password":
		return "must be at least 8 characters"
	default:
		return fmt.Sprintf("failed validation (%s)", e.Tag())
	}
}
