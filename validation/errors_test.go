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

//go:build !integration

package validation

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_NilValue(t *testing.T) {
	t.Parallel()
	err := Validate(t.Context(), nil)
	require.Error(t, err, "expected error for nil value")
}

func TestValidate_NilPointer(t *testing.T) {
	t.Parallel()
	var ptr *struct {
		Name string `json:"name" validate:"required"`
	}
	err := Validate(t.Context(), ptr)
	require.Error(t, err, "expected error for nil pointer")
	var verr *Error
	require.ErrorAs(t, err, &verr, "expected validation.Error")
	assert.Equal(t, "nil_pointer", verr.Fields[0].Code)
}

func TestValidationErrors_HasErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func() Error
		expected bool
	}{
		{
			name: "empty errors should not have errors",
			setup: func() Error {
				return Error{}
			},
			expected: false,
		},
		{
			name: "should have errors after adding one",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)

				return verr
			},
			expected: true,
		},
		{
			name: "should have errors with multiple fields",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)
				verr.Add("email", "email", "invalid email", nil)

				return verr
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			verr := tt.setup()
			assert.Equal(t, tt.expected, verr.HasErrors())
		})
	}
}

func TestValidationErrors_Is(t *testing.T) {
	t.Parallel()
	var verr Error
	verr.Add("name", "required", "is required", nil)
	verr.Add("email", "email", "must be email", nil)
	verr.Add("age", "required", "age is required", nil) // Same code, different field

	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		{"should find 'required' code", "required", true},
		{"should find 'email' code", "email", true},
		{"should not find nonexistent code", "nonexistent", false},
		{"should find code even if multiple fields have it", "required", true},
		{"case sensitive code lookup", "Required", false},
		{"empty code should not match", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, verr.HasCode(tt.code))
		})
	}
}

func TestValidationErrors_Sort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func() Error
		validate func(t *testing.T, verr Error)
	}{
		{
			name: "sort by path then code",
			setup: func() Error {
				var verr Error
				verr.Add("z", "code1", "msg1", nil)
				verr.Add("a", "code2", "msg2", nil)
				verr.Add("a", "code1", "msg3", nil)
				verr.Sort()

				return verr
			},
			validate: func(t *testing.T, verr Error) {
				t.Helper()
				assert.Equal(t, "a", verr.Fields[0].Path)
				assert.Equal(t, "code1", verr.Fields[0].Code)
				assert.Equal(t, "a", verr.Fields[1].Path)
				assert.Equal(t, "code2", verr.Fields[1].Code)
				assert.Equal(t, "z", verr.Fields[2].Path)
				assert.Equal(t, "code1", verr.Fields[2].Code)
			},
		},
		{
			name: "sort empty errors",
			setup: func() Error {
				var verr Error
				verr.Sort()

				return verr
			},
			validate: func(t *testing.T, verr Error) {
				t.Helper()
				assert.Empty(t, verr.Fields)
			},
		},
		{
			name: "sort single field",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)
				verr.Sort()

				return verr
			},
			validate: func(t *testing.T, verr Error) {
				t.Helper()
				assert.Len(t, verr.Fields, 1)
				assert.Equal(t, "name", verr.Fields[0].Path)
			},
		},
		{
			name: "sort with same path and code",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "msg1", nil)
				verr.Add("name", "required", "msg2", nil)
				verr.Sort()

				return verr
			},
			validate: func(t *testing.T, verr Error) {
				t.Helper()
				assert.Len(t, verr.Fields, 2)
				assert.Equal(t, "name", verr.Fields[0].Path)
				assert.Equal(t, "name", verr.Fields[1].Path)
			},
		},
		{
			name: "sort with empty paths",
			setup: func() Error {
				var verr Error
				verr.Add("", "code1", "msg1", nil)
				verr.Add("a", "code2", "msg2", nil)
				verr.Add("", "code2", "msg3", nil)
				verr.Sort()

				return verr
			},
			validate: func(t *testing.T, verr Error) {
				t.Helper()
				assert.Empty(t, verr.Fields[0].Path)
				assert.Equal(t, "code1", verr.Fields[0].Code)
				assert.Empty(t, verr.Fields[1].Path)
				assert.Equal(t, "code2", verr.Fields[1].Code)
				assert.Equal(t, "a", verr.Fields[2].Path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			verr := tt.setup()
			tt.validate(t, verr)
		})
	}
}

func TestValidationErrors_Unwrap(t *testing.T) {
	t.Parallel()
	var verr Error
	verr.Add("name", "required", "is required", nil)

	err := verr.Unwrap()
	assert.ErrorIs(t, err, ErrValidation, "Unwrap should return ErrValidation")
}

func TestFieldError_Error(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fe       FieldError
		expected string
	}{
		{
			name: "with path",
			fe: FieldError{
				Path:    "name",
				Code:    "required",
				Message: "is required",
			},
			expected: "name: is required",
		},
		{
			name: "without path",
			fe: FieldError{
				Code:    "required",
				Message: "is required",
			},
			expected: "is required",
		},
		{
			name: "with nested path",
			fe: FieldError{
				Path:    "user.address.city",
				Code:    "required",
				Message: "city is required",
			},
			expected: "user.address.city: city is required",
		},
		{
			name: "with array index path",
			fe: FieldError{
				Path:    "items.0.name",
				Code:    "required",
				Message: "name is required",
			},
			expected: "items.0.name: name is required",
		},
		{
			name: "with empty message",
			fe: FieldError{
				Path:    "name",
				Code:    "required",
				Message: "",
			},
			expected: "name: ",
		},
		{
			name: "with meta information",
			fe: FieldError{
				Path:    "age",
				Code:    "min",
				Message: "age must be at least 18",
				Meta:    map[string]any{"min": 18, "actual": 15},
			},
			expected: "age: age must be at least 18",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.fe.Error())
		})
	}
}

func TestFieldError_Unwrap(t *testing.T) {
	t.Parallel()
	fe := FieldError{
		Path:    "name",
		Code:    "required",
		Message: "is required",
	}

	err := fe.Unwrap()
	assert.ErrorIs(t, err, ErrValidation, "Unwrap should return ErrValidation")
}

func TestFieldError_HTTPStatus(t *testing.T) {
	t.Parallel()
	fe := FieldError{
		Path:    "name",
		Code:    "required",
		Message: "is required",
	}
	assert.Equal(t, 422, fe.HTTPStatus(), "FieldError.HTTPStatus should return 422 Unprocessable Entity")
}

func TestError_HTTPStatus(t *testing.T) {
	t.Parallel()
	var verr Error
	verr.Add("name", "required", "is required", nil)
	assert.Equal(t, 422, verr.HTTPStatus(), "Error.HTTPStatus should return 422 Unprocessable Entity")
}

func TestError_Details(t *testing.T) {
	t.Parallel()
	var verr Error
	verr.Add("name", "required", "is required", nil)
	verr.Add("email", "email", "invalid email", nil)
	details := verr.Details()
	require.NotNil(t, details)
	fields, ok := details.([]FieldError)
	require.True(t, ok, "Details should return []FieldError")
	assert.Len(t, fields, 2)
	assert.Equal(t, "name", fields[0].Path)
	assert.Equal(t, "email", fields[1].Path)
}

func TestError_Code(t *testing.T) {
	t.Parallel()
	var verr Error
	verr.Add("name", "required", "is required", nil)
	assert.Equal(t, "validation_error", verr.Code(), "Error.Code should return validation_error")
}

func TestValidationErrors_Add(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		path     string
		code     string
		message  string
		meta     map[string]any
		validate func(t *testing.T, verr Error)
	}{
		{
			name:    "add error without meta",
			path:    "name",
			code:    "required",
			message: "is required",
			meta:    nil,
			validate: func(t *testing.T, verr Error) {
				t.Helper()
				assert.Len(t, verr.Fields, 1)
				assert.Equal(t, "name", verr.Fields[0].Path)
				assert.Equal(t, "required", verr.Fields[0].Code)
				assert.Nil(t, verr.Fields[0].Meta)
			},
		},
		{
			name:    "add error with meta",
			path:    "age",
			code:    "min",
			message: "must be at least 18",
			meta:    map[string]any{"min": 18, "actual": 15},
			validate: func(t *testing.T, verr Error) {
				t.Helper()
				assert.Len(t, verr.Fields, 1)
				assert.Equal(t, "age", verr.Fields[0].Path)
				assert.Equal(t, "min", verr.Fields[0].Code)
				assert.Equal(t, 18, verr.Fields[0].Meta["min"])
				assert.Equal(t, 15, verr.Fields[0].Meta["actual"])
			},
		},
		{
			name:    "add error with empty path",
			path:    "",
			code:    "general",
			message: "general error",
			meta:    nil,
			validate: func(t *testing.T, verr Error) {
				t.Helper()
				assert.Len(t, verr.Fields, 1)
				assert.Empty(t, verr.Fields[0].Path)
			},
		},
		{
			name:    "add multiple errors",
			path:    "email",
			code:    "email",
			message: "invalid email",
			meta:    nil,
			validate: func(t *testing.T, verr Error) {
				t.Helper()
				assert.Len(t, verr.Fields, 2)
				assert.Equal(t, "name", verr.Fields[0].Path)
				assert.Equal(t, "email", verr.Fields[1].Path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var verr Error
			if tt.name == "add multiple errors" {
				// For this test, add a first error
				verr.Add("name", "required", "is required", nil)
			}
			verr.Add(tt.path, tt.code, tt.message, tt.meta)
			tt.validate(t, verr)
		})
	}
}

func TestValidationErrors_AddError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		setup         func() error
		expectedCount int
		expectedCode  string
	}{
		{
			name: "add FieldError",
			setup: func() error {
				return FieldError{
					Path:    "name",
					Code:    "required",
					Message: "is required",
				}
			},
			expectedCount: 1,
			expectedCode:  "required",
		},
		{
			name: "add validation.Error",
			setup: func() error {
				var verr Error
				verr.Add("email", "email", "invalid email", nil)

				return verr
			},
			expectedCount: 1,
			expectedCode:  "email",
		},
		{
			name: "add generic error",
			setup: func() error {
				return errGenericError
			},
			expectedCount: 1,
			expectedCode:  "validation_error",
		},
		{
			name: "add nil error (should be ignored)",
			setup: func() error {
				return nil
			},
			expectedCount: 0,
			expectedCode:  "",
		},
		{
			name: "add value Error propagates Truncated",
			setup: func() error {
				var valErr Error
				valErr.Add("a", "code1", "msg1", nil)
				valErr.Add("b", "code2", "msg2", nil)
				valErr.Truncated = true
				return valErr
			},
			expectedCount: 2,
			expectedCode:  "code2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var verr Error
			err := tt.setup()
			verr.AddError(err)
			assert.Len(t, verr.Fields, tt.expectedCount)
			if tt.expectedCode != "" && tt.expectedCount > 0 {
				assert.Equal(t, tt.expectedCode, verr.Fields[tt.expectedCount-1].Code)
			}
			if tt.name == "add value Error propagates Truncated" {
				assert.True(t, verr.Truncated, "Truncated should be propagated from value Error")
			}
		})
	}

	// Test cumulative addition
	t.Run("cumulative addition of multiple errors", func(t *testing.T) {
		t.Parallel()
		var verr Error
		verr.AddError(FieldError{Path: "name", Code: "required", Message: "is required"})
		assert.Len(t, verr.Fields, 1)

		var verr2 Error
		verr2.Add("email", "email", "invalid email", nil)
		verr.AddError(verr2)
		assert.Len(t, verr.Fields, 2)

		verr.AddError(errGenericError)
		assert.Len(t, verr.Fields, 3)
		assert.Equal(t, "validation_error", verr.Fields[2].Code)

		initialCount := len(verr.Fields)
		verr.AddError(nil)
		assert.Len(t, verr.Fields, initialCount, "nil error should be ignored")
	})
}

func TestValidationErrors_Error(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		setup       func() Error
		checkEmpty  bool
		contains    []string
		notContains []string
	}{
		{
			name: "empty errors should return empty string",
			setup: func() Error {
				return Error{}
			},
			checkEmpty: true,
		},
		{
			name: "single error should contain field name",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)

				return verr
			},
			contains: []string{"name"},
		},
		{
			name: "single error with empty path",
			setup: func() Error {
				var verr Error
				verr.Add("", "general", "general error", nil)

				return verr
			},
			contains:    []string{"general error"},
			notContains: []string{":"},
		},
		{
			name: "multiple errors should have 'validation failed' prefix",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)
				verr.Add("email", "email", "invalid email", nil)

				return verr
			},
			contains: []string{"validation failed", "name", "email"},
		},
		{
			name: "truncated errors should indicate truncation",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)
				verr.Add("email", "email", "invalid email", nil)
				verr.Truncated = true

				return verr
			},
			contains: []string{"truncated"},
		},
		{
			name: "single error should not have 'validation failed' prefix",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)

				return verr
			},
			contains:    []string{"name"},
			notContains: []string{"validation failed"},
		},
		{
			name: "multiple errors with nested paths",
			setup: func() Error {
				var verr Error
				verr.Add("user.name", "required", "is required", nil)
				verr.Add("user.email", "email", "invalid email", nil)

				return verr
			},
			contains: []string{"validation failed", "user.name", "user.email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			verr := tt.setup()
			msg := verr.Error()

			if tt.checkEmpty {
				assert.Empty(t, msg)
			} else {
				assert.NotEmpty(t, msg)
			}

			for _, substr := range tt.contains {
				assert.Contains(t, msg, substr)
			}

			for _, substr := range tt.notContains {
				assert.NotContains(t, msg, substr)
			}
		})
	}
}

func TestPresenceMap_Has(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pm       PresenceMap
		key      string
		expected bool
	}{
		{
			name: "should have 'name'",
			pm: PresenceMap{
				"name":  true,
				"email": true,
			},
			key:      "name",
			expected: true,
		},
		{
			name: "should have 'email'",
			pm: PresenceMap{
				"name":  true,
				"email": true,
			},
			key:      "email",
			expected: true,
		},
		{
			name: "should not have 'nonexistent'",
			pm: PresenceMap{
				"name":  true,
				"email": true,
			},
			key:      "nonexistent",
			expected: false,
		},
		{
			name:     "empty map should return false",
			pm:       PresenceMap{},
			key:      "anything",
			expected: false,
		},
		{
			name: "should handle nested paths",
			pm: PresenceMap{
				"user.name": true,
			},
			key:      "user.name",
			expected: true,
		},
		{
			name: "should handle empty key",
			pm: PresenceMap{
				"": true,
			},
			key:      "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.pm.Has(tt.key))
		})
	}
}

func TestPresenceMap_HasPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pm       PresenceMap
		prefix   string
		expected bool
	}{
		{
			name: "should have 'address' prefix",
			pm: PresenceMap{
				"address":      true,
				"address.city": true,
				"address.zip":  true,
				"items":        true,
				"items.0":      true,
				"items.0.name": true,
			},
			prefix:   "address",
			expected: true,
		},
		{
			name: "should have 'address.city' prefix",
			pm: PresenceMap{
				"address":      true,
				"address.city": true,
				"address.zip":  true,
			},
			prefix:   "address.city",
			expected: true,
		},
		{
			name: "should have 'items' prefix",
			pm: PresenceMap{
				"items":        true,
				"items.0":      true,
				"items.0.name": true,
			},
			prefix:   "items",
			expected: true,
		},
		{
			name: "should have 'items.0' prefix",
			pm: PresenceMap{
				"items":        true,
				"items.0":      true,
				"items.0.name": true,
			},
			prefix:   "items.0",
			expected: true,
		},
		{
			name: "should not have 'nonexistent' prefix",
			pm: PresenceMap{
				"address": true,
			},
			prefix:   "nonexistent",
			expected: false,
		},
		{
			name:     "empty map should return false",
			pm:       PresenceMap{},
			prefix:   "anything",
			expected: false,
		},
		{
			name: "should handle empty prefix with empty key",
			pm: PresenceMap{
				"": true,
			},
			prefix:   "",
			expected: true, // Empty prefix matches empty key
		},
		{
			name: "empty prefix should not match non-empty keys",
			pm: PresenceMap{
				"name": true,
			},
			prefix:   "",
			expected: false, // Empty prefix doesn't match "name"
		},
		{
			name: "partial match should not count",
			pm: PresenceMap{
				"address": true,
			},
			prefix:   "addr", // Partial match, not a prefix
			expected: false,
		},
		{
			name:     "nil map should return false",
			pm:       (PresenceMap)(nil),
			prefix:   "any",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.pm.HasPrefix(tt.prefix))
		})
	}
}

func TestValidationErrors_Has(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func() Error
		path     string
		expected bool
	}{
		{
			name: "should find existing path",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)

				return verr
			},
			path:     "name",
			expected: true,
		},
		{
			name: "should not find nonexistent path",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)

				return verr
			},
			path:     "email",
			expected: false,
		},
		{
			name: "should find nested path",
			setup: func() Error {
				var verr Error
				verr.Add("user.address.city", "required", "is required", nil)

				return verr
			},
			path:     "user.address.city",
			expected: true,
		},
		{
			name: "should not find path in empty errors",
			setup: func() Error {
				return Error{}
			},
			path:     "name",
			expected: false,
		},
		{
			name: "should handle empty path",
			setup: func() Error {
				var verr Error
				verr.Add("", "general", "general error", nil)

				return verr
			},
			path:     "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			verr := tt.setup()
			assert.Equal(t, tt.expected, verr.Has(tt.path))
		})
	}
}

func TestValidationErrors_GetField(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func() Error
		path     string
		expected *FieldError
	}{
		{
			name: "should get existing field",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)

				return verr
			},
			path: "name",
			expected: &FieldError{
				Path:    "name",
				Code:    "required",
				Message: "is required",
			},
		},
		{
			name: "should return nil for nonexistent path",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)

				return verr
			},
			path:     "email",
			expected: nil,
		},
		{
			name: "should get first field when multiple exist",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)
				verr.Add("name", "min", "must be at least 3", nil)

				return verr
			},
			path: "name",
			expected: &FieldError{
				Path:    "name",
				Code:    "required",
				Message: "is required",
			},
		},
		{
			name: "should return nil for empty errors",
			setup: func() Error {
				return Error{}
			},
			path:     "name",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			verr := tt.setup()
			result := verr.GetField(tt.path)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Path, result.Path)
				assert.Equal(t, tt.expected.Code, result.Code)
				assert.Equal(t, tt.expected.Message, result.Message)
			}
		})
	}
}

func TestPresenceMap_LeafPaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pm       PresenceMap
		expected map[string]bool
	}{
		{
			name: "basic leaf paths",
			pm: PresenceMap{
				"name":         true,
				"address":      true,
				"address.city": true,
				"address.zip":  true,
				"items":        true,
				"items.0":      true,
				"items.0.name": true,
				"items.1":      true,
			},
			expected: map[string]bool{
				"name":         true,
				"address.city": true,
				"address.zip":  true,
				"items.0.name": true,
				"items.1":      true,
			},
		},
		{
			name:     "empty map",
			pm:       PresenceMap{},
			expected: map[string]bool{},
		},
		{
			name: "only leaf nodes",
			pm: PresenceMap{
				"name":  true,
				"email": true,
			},
			expected: map[string]bool{
				"name":  true,
				"email": true,
			},
		},
		{
			name: "deeply nested paths",
			pm: PresenceMap{
				"user":                true,
				"user.profile":        true,
				"user.profile.name":   true,
				"user.profile.email":  true,
				"user.settings":       true,
				"user.settings.theme": true,
			},
			expected: map[string]bool{
				"user.profile.name":   true,
				"user.profile.email":  true,
				"user.settings.theme": true,
			},
		},
		{
			name: "array indices",
			pm: PresenceMap{
				"items":      true,
				"items.0":    true,
				"items.0.id": true,
				"items.1":    true,
				"items.1.id": true,
				"items.2":    true,
			},
			expected: map[string]bool{
				"items.0.id": true,
				"items.1.id": true,
				"items.2":    true,
			},
		},
		{
			name:     "nil map returns nil",
			pm:       (PresenceMap)(nil),
			expected: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			leaves := tt.pm.LeafPaths()
			if tt.pm == nil {
				assert.Nil(t, leaves)
				return
			}
			assert.Len(t, leaves, len(tt.expected), "expected %d leaves", len(tt.expected))

			for _, leaf := range leaves {
				assert.True(t, tt.expected[leaf], "unexpected leaf: %q", leaf)
			}

			// Verify all expected leaves are present
			for expectedLeaf := range tt.expected {
				assert.True(t, slices.Contains(leaves, expectedLeaf), "expected leaf %q not found", expectedLeaf)
			}
		})
	}
}

func TestComputePresence_MaxDepth(t *testing.T) {
	t.Parallel()
	// Build JSON with depth > maxRecursionDepth to hit the depth guard in markPresence
	inner := "{}"
	for range maxRecursionDepth + 2 {
		inner = `{"x":` + inner + `}`
	}
	rawJSON := []byte(inner)
	pm, err := ComputePresence(rawJSON)
	require.NoError(t, err)
	// Should not panic; may have limited paths due to depth guard
	assert.NotNil(t, pm)
}

func TestValidatorInterface_WithContext(t *testing.T) {
	t.Parallel()
	type TestStruct struct {
		Name string `json:"name"`
	}

	// Test struct without Validate method should pass
	impl := &TestStruct{Name: "test"}
	ctx := t.Context()
	err := Validate(ctx, impl, WithStrategy(StrategyInterface), WithContext(ctx))
	// Interface validation should pass (no Validate method)
	assert.NoError(t, err)
}

func TestValidationStrategy_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		strategy Strategy
	}{
		{"StrategyAuto", StrategyAuto},
		{"StrategyTags", StrategyTags},
		{"StrategyJSONSchema", StrategyJSONSchema},
		{"StrategyInterface", StrategyInterface},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.GreaterOrEqual(t, int(tt.strategy), 0, "strategy should be >= 0")
			assert.LessOrEqual(t, int(tt.strategy), int(StrategyInterface), "strategy should be <= StrategyInterface")
		})
	}
}
