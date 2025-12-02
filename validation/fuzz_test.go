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
	"testing"
)

// FuzzComputePresence tests the ComputePresence function with random JSON inputs.
// It should never panic, even with malformed or adversarial input.
func FuzzComputePresence(f *testing.F) {
	// Seed corpus with various JSON structures
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"key": "value"}`))
	f.Add([]byte(`{"nested": {"key": "value"}}`))
	f.Add([]byte(`{"array": [1, 2, 3]}`))
	f.Add([]byte(`{"mixed": {"arr": [{"id": 1}, {"id": 2}]}}`))
	f.Add([]byte(`{"deep": {"level1": {"level2": {"level3": "value"}}}}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`[{"id": 1}, {"id": 2}]`))
	f.Add([]byte(`"string"`))
	f.Add([]byte(`123`))
	f.Add([]byte(`true`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add([]byte(`{invalid`))
	f.Add([]byte(`{"unclosed": "string`))
	f.Add([]byte(`{"emoji": "🎉"}`))
	f.Add([]byte(`{"unicode": "日本語"}`))
	f.Add([]byte(`{"special": "tab\there"}`))
	f.Add([]byte(`{"newline": "line1\nline2"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// ComputePresence should never panic
		pm, err := ComputePresence(data)

		// If no error, presence map should be usable
		if err == nil && pm != nil {
			// These operations should not panic
			_ = pm.Has("any.path")
			_ = pm.HasPrefix("any")
			_ = pm.LeafPaths()
		}
	})
}

// FuzzValidate tests the Validate function with random struct field values.
// It should never panic and should return appropriate errors.
func FuzzValidate(f *testing.F) {
	// Seed corpus with various string inputs
	f.Add("", "")
	f.Add("valid", "valid@example.com")
	f.Add("a", "invalid-email")
	f.Add("long string with spaces", "email@domain.co.uk")
	f.Add("unicode: 日本語", "test@日本語.com")
	f.Add("emoji: 🎉", "emoji@🎉.com")
	f.Add("special\tchar", "tab\there@test.com")
	f.Add("newline\nchar", "newline\n@test.com")
	f.Add("<script>alert('xss')</script>", "xss@<script>.com")
	f.Add("very_long_string_that_exceeds_normal_length_limits_and_might_cause_issues_with_validation_rules_or_memory_allocation", "very_long_email_address_that_exceeds_normal_length_limits@very_long_domain_name_that_also_exceeds_normal_limits.com")

	f.Fuzz(func(t *testing.T, name, email string) {
		type User struct {
			Name  string `json:"name" validate:"required"`
			Email string `json:"email" validate:"email"`
		}

		user := &User{Name: name, Email: email}
		ctx := context.Background()

		// Validate should never panic
		err := Validate(ctx, user, WithStrategy(StrategyTags))

		// If there's an error, it should be a validation error
		if err != nil {
			var verr *Error
			if !isValidationError(err) {
				t.Errorf("unexpected error type: %T", err)
			}
			_ = verr // Suppress unused variable warning
		}
	})
}

// FuzzValidateJSONSchema tests JSON Schema validation with random inputs.
func FuzzValidateJSONSchema(f *testing.F) {
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"value": {"type": "number"}
		}
	}`

	// Seed corpus
	f.Add(`{"name": "test", "value": 123}`)
	f.Add(`{"name": "", "value": 0}`)
	f.Add(`{"name": null}`)
	f.Add(`{}`)
	f.Add(`{"extra": "field"}`)
	f.Add(`{"name": 123}`)       // Wrong type
	f.Add(`{"value": "string"}`) // Wrong type
	f.Add(`invalid json`)
	f.Add(``)

	f.Fuzz(func(t *testing.T, jsonInput string) {
		// Try to parse the JSON
		var data map[string]any
		if err := json.Unmarshal([]byte(jsonInput), &data); err != nil {
			// Invalid JSON - skip validation but don't panic
			return
		}

		ctx := context.Background()

		// Validation should never panic
		_ = Validate(ctx, &data,
			WithStrategy(StrategyJSONSchema),
			WithCustomSchema("fuzz-schema", schema),
		)
	})
}

// FuzzPresenceMapOperations tests PresenceMap methods with random paths.
func FuzzPresenceMapOperations(f *testing.F) {
	// Seed corpus with various path patterns
	f.Add("simple")
	f.Add("nested.path")
	f.Add("deeply.nested.path.here")
	f.Add("array.0.field")
	f.Add("array.999.field")
	f.Add("")
	f.Add(".")
	f.Add("..")
	f.Add("path..double")
	f.Add("unicode.日本語.field")
	f.Add("emoji.🎉.field")
	f.Add("special\t.char")
	f.Add("special\n.char")

	f.Fuzz(func(t *testing.T, path string) {
		pm := PresenceMap{
			"name":         true,
			"address":      true,
			"address.city": true,
			"items":        true,
			"items.0":      true,
			"items.0.name": true,
		}

		// These operations should never panic
		_ = pm.Has(path)
		_ = pm.HasPrefix(path)

		// Add the fuzzed path and test again
		pm[path] = true
		_ = pm.Has(path)
		_ = pm.HasPrefix(path)
		_ = pm.LeafPaths()
	})
}

// FuzzValidationError tests Error type methods with random inputs.
func FuzzValidationError(f *testing.F) {
	f.Add("field.path", "error_code", "Error message")
	f.Add("", "", "")
	f.Add("nested.deeply.path", "required", "field is required")
	f.Add("array.0.field", "min", "must be at least 5")
	f.Add("unicode.日本語", "custom", "カスタムエラー")
	f.Add("emoji.🎉", "emoji", "🎉 error")

	f.Fuzz(func(t *testing.T, path, code, message string) {
		var verr Error

		// Add should never panic
		verr.Add(path, code, message, nil)
		verr.Add(path, code, message, map[string]any{"key": "value"})

		// These operations should never panic
		_ = verr.Error()
		_ = verr.HasErrors()
		_ = verr.HasCode(code)
		_ = verr.Has(path)
		_ = verr.GetField(path)
		_ = verr.Unwrap()

		verr.Sort()
	})
}

// FuzzFieldError tests FieldError type methods.
func FuzzFieldError(f *testing.F) {
	f.Add("path", "code", "message")
	f.Add("", "", "")
	f.Add("long.nested.path.with.many.segments", "validation_error", "A very long error message that exceeds normal lengths")

	f.Fuzz(func(t *testing.T, path, code, message string) {
		fe := FieldError{
			Path:    path,
			Code:    code,
			Message: message,
		}

		// These should never panic
		_ = fe.Error()
		_ = fe.Unwrap()
	})
}

// isValidationError checks if an error is a validation-related error.
// It returns true for any error that is expected from the validation package.
func isValidationError(err error) bool {
	if err == nil {
		return false
	}

	// Check for sentinel errors
	if err == ErrValidation ||
		err == ErrValidationFailed ||
		err == ErrCannotValidateNilValue ||
		err == ErrInvalidType {
		return true
	}

	// Check for validation.Error type (most common case)
	switch err.(type) {
	case *Error, Error, FieldError:
		return true
	}

	// Check if it wraps ErrValidation
	type unwrapper interface {
		Unwrap() error
	}
	if u, ok := err.(unwrapper); ok {
		if unwrapped := u.Unwrap(); unwrapped != nil {
			return isValidationError(unwrapped)
		}
	}

	return false
}
