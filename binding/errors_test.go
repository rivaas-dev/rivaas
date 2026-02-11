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

package binding

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBindError_Unwrap tests the Unwrap method
func TestBindError_Unwrap(t *testing.T) {
	t.Parallel()

	originalErr := &BindError{
		Field:  "age",
		Value:  "invalid",
		Type:   reflect.TypeFor[int](),
		Source: SourceForm,
		Err:    nil,
	}

	innerErr := &BindError{
		Field:  "nested",
		Value:  "bad",
		Type:   reflect.TypeFor[string](),
		Source: SourceJSON,
	}

	outerErr := &BindError{
		Field:  "age",
		Value:  "invalid",
		Type:   reflect.TypeFor[int](),
		Source: SourceForm,
		Err:    innerErr,
	}

	unwrapped := outerErr.Unwrap()
	require.ErrorIs(t, unwrapped, innerErr)

	assert.NoError(t, originalErr.Unwrap())
}

// TestBindError_Hint tests the hint method for contextual error messages.
func TestBindError_Hint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		bindError    *BindError
		expectHint   string
		containsHint bool
	}{
		{
			name: "integer with decimal point",
			bindError: &BindError{
				Field:  "age",
				Source: SourceQuery,
				Value:  "25.5",
				Type:   reflect.TypeFor[int](),
				Err:    assert.AnError,
			},
			expectHint:   "use float type for decimal values",
			containsHint: true,
		},
		{
			name: "time parsing failed",
			bindError: &BindError{
				Field:  "created_at",
				Source: SourceQuery,
				Value:  "2024/12/25",
				Type:   timeType,
				Err:    assert.AnError,
			},
			expectHint:   "use RFC3339 format",
			containsHint: true,
		},
		{
			name: "duration parsing failed",
			bindError: &BindError{
				Field:  "timeout",
				Source: SourceQuery,
				Value:  "invalid",
				Type:   durationType,
				Err:    assert.AnError,
			},
			expectHint:   "use Go duration format",
			containsHint: true,
		},
		{
			name: "boolean with unexpected value",
			bindError: &BindError{
				Field:  "enabled",
				Source: SourceQuery,
				Value:  "maybe",
				Type:   reflect.TypeFor[bool](),
				Err:    assert.AnError,
			},
			expectHint:   "accepted values: true/false",
			containsHint: true,
		},
		{
			name: "slice with CSV",
			bindError: &BindError{
				Field:  "tags",
				Source: SourceQuery,
				Value:  "tag1,tag2,tag3",
				Type:   reflect.TypeFor[[]string](),
				Err:    assert.AnError,
			},
			expectHint:   "for CSV values",
			containsHint: true,
		},
		{
			name: "map without proper notation",
			bindError: &BindError{
				Field:  "metadata",
				Source: SourceQuery,
				Value:  "key=value",
				Type:   reflect.TypeFor[map[string]string](),
				Err:    assert.AnError,
			},
			expectHint:   "use dot notation",
			containsHint: true,
		},
		{
			name: "pointer to int with decimal",
			bindError: &BindError{
				Field:  "age",
				Source: SourceQuery,
				Value:  "25.5",
				Type:   reflect.TypeFor[*int](),
				Err:    assert.AnError,
			},
			expectHint:   "use float pointer type",
			containsHint: true,
		},
		{
			name: "no hint - string type",
			bindError: &BindError{
				Field:  "name",
				Source: SourceQuery,
				Value:  "john",
				Type:   reflect.TypeFor[string](),
				Err:    assert.AnError,
			},
			expectHint:   "",
			containsHint: false,
		},
		{
			name: "no hint - nil type",
			bindError: &BindError{
				Field:  "field",
				Source: SourceQuery,
				Value:  "value",
				Type:   nil,
				Err:    assert.AnError,
			},
			expectHint:   "",
			containsHint: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hint := tt.bindError.hint()
			errorMsg := tt.bindError.Error()

			if tt.containsHint {
				assert.Contains(t, hint, tt.expectHint, "hint should contain expected text")
				assert.Contains(t, errorMsg, "(hint:", "error message should include hint")
				assert.Contains(t, errorMsg, tt.expectHint, "error message should contain hint text")
			} else {
				assert.Empty(t, hint, "hint should be empty")
				assert.NotContains(t, errorMsg, "(hint:", "error message should not include hint")
			}
		})
	}
}

// TestBindError_Error_WithHints tests the complete error message with hints.
func TestBindError_Error_WithHints(t *testing.T) {
	t.Parallel()

	// Integer field with decimal value
	err := &BindError{
		Field:  "quantity",
		Source: SourceQuery,
		Value:  "10.5",
		Type:   reflect.TypeFor[int](),
		Err:    assert.AnError,
	}
	errorMsg := err.Error()
	assert.Contains(t, errorMsg, `binding field "quantity" (query)`)
	assert.Contains(t, errorMsg, "10.5")
	assert.Contains(t, errorMsg, "(hint: use float type for decimal values)")

	// Time field with wrong format
	err = &BindError{
		Field:  "timestamp",
		Source: SourceForm,
		Value:  "12/25/2024",
		Type:   timeType,
		Err:    assert.AnError,
	}
	errorMsg = err.Error()
	assert.Contains(t, errorMsg, `binding field "timestamp" (form)`)
	assert.Contains(t, errorMsg, "(hint: use RFC3339 format")

	// Boolean field with invalid value
	err = &BindError{
		Field:  "active",
		Source: SourceHeader,
		Value:  "maybe",
		Type:   reflect.TypeFor[bool](),
		Err:    assert.AnError,
	}
	errorMsg = err.Error()
	assert.Contains(t, errorMsg, `binding field "active" (header)`)
	assert.Contains(t, errorMsg, "(hint: accepted values")
}

// TestBindError_Error_WithReason tests error messages with custom reasons.
func TestBindError_Error_WithReason(t *testing.T) {
	t.Parallel()

	// Test with custom reason (no hint expected since value doesn't match any hint patterns)
	err := &BindError{
		Field:  "id",
		Source: SourcePath,
		Value:  "abc",
		Type:   reflect.TypeFor[int](),
		Reason: "custom reason message",
	}

	errorMsg := err.Error()
	assert.Contains(t, errorMsg, `binding field "id" (path)`)
	assert.Contains(t, errorMsg, "custom reason message")

	// Test with custom reason and a value that triggers a hint
	err2 := &BindError{
		Field:  "quantity",
		Source: SourceQuery,
		Value:  "10.5",
		Type:   reflect.TypeFor[int](),
		Reason: "value must be a whole number",
	}

	errorMsg2 := err2.Error()
	assert.Contains(t, errorMsg2, "value must be a whole number")
	assert.Contains(t, errorMsg2, "(hint: use float type for decimal values)")
}

// TestIsIntType tests the isIntType helper function.
func TestIsIntType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		typ    reflect.Type
		expect bool
	}{
		{"int", reflect.TypeFor[int](), true},
		{"int8", reflect.TypeFor[int8](), true},
		{"int16", reflect.TypeFor[int16](), true},
		{"int32", reflect.TypeFor[int32](), true},
		{"int64", reflect.TypeFor[int64](), true},
		{"uint", reflect.TypeFor[uint](), true},
		{"uint8", reflect.TypeFor[uint8](), true},
		{"uint16", reflect.TypeFor[uint16](), true},
		{"uint32", reflect.TypeFor[uint32](), true},
		{"uint64", reflect.TypeFor[uint64](), true},
		{"float32", reflect.TypeFor[float32](), false},
		{"float64", reflect.TypeFor[float64](), false},
		{"string", reflect.TypeFor[string](), false},
		{"bool", reflect.TypeFor[bool](), false},
		{"nil type", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := isIntType(tt.typ)
			assert.Equal(t, tt.expect, result)
		})
	}
}

// TestSource_String tests string representation of all Source constants.
func TestSource_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		source   Source
		expected string
	}{
		{"query", SourceQuery, "query"},
		{"path", SourcePath, "path"},
		{"form", SourceForm, "form"},
		{"header", SourceHeader, "header"},
		{"cookie", SourceCookie, "cookie"},
		{"json", SourceJSON, "json"},
		{"xml", SourceXML, "xml"},
		{"yaml", SourceYAML, "yaml"},
		{"toml", SourceTOML, "toml"},
		{"msgpack", SourceMsgPack, "msgpack"},
		{"proto", SourceProto, "proto"},
		{"multipart", SourceMultipart, "multipart"},
		{"unknown", SourceUnknown, "unknown"},
		{"invalid value yields unknown", Source(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.source.String()
			assert.Equal(t, tt.expected, got)
		})
	}
}

// TestBindError_HTTPStatus_Code_IsType tests BindError interface methods.
func TestBindError_HTTPStatus_Code_IsType(t *testing.T) {
	t.Parallel()

	t.Run("with underlying error", func(t *testing.T) {
		t.Parallel()

		err := &BindError{
			Field:  "age",
			Source: SourceQuery,
			Value:  "invalid",
			Type:   reflect.TypeFor[int](),
			Err:    assert.AnError,
		}
		assert.Equal(t, 400, err.HTTPStatus())
		assert.Equal(t, "binding_error", err.Code())
		assert.True(t, err.IsType())
	})

	t.Run("without underlying error", func(t *testing.T) {
		t.Parallel()

		err := &BindError{
			Field:  "name",
			Source: SourceForm,
			Err:    nil,
		}
		assert.Equal(t, 400, err.HTTPStatus())
		assert.Equal(t, "binding_error", err.Code())
		assert.False(t, err.IsType())
	})
}

// TestUnknownFieldError_Error tests error message for single and multiple fields.
func TestUnknownFieldError_Error(t *testing.T) {
	t.Parallel()

	t.Run("single field", func(t *testing.T) {
		t.Parallel()

		err := &UnknownFieldError{Fields: []string{"extra"}}
		msg := err.Error()
		assert.Contains(t, msg, "unknown field")
		assert.Contains(t, msg, "extra")
	})

	t.Run("multiple fields", func(t *testing.T) {
		t.Parallel()

		err := &UnknownFieldError{Fields: []string{"a", "b"}}
		msg := err.Error()
		assert.Contains(t, msg, "unknown fields")
		assert.Contains(t, msg, "a")
		assert.Contains(t, msg, "b")
	})
}

// TestUnknownFieldError_HTTPStatus_Code tests UnknownFieldError interface methods.
func TestUnknownFieldError_HTTPStatus_Code(t *testing.T) {
	t.Parallel()

	err := &UnknownFieldError{Fields: []string{"x"}}
	assert.Equal(t, 400, err.HTTPStatus())
	assert.Equal(t, "unknown_field", err.Code())
}

// TestMultiError_Error tests error message for zero one and multiple errors.
func TestMultiError_Error(t *testing.T) {
	t.Parallel()

	t.Run("zero errors returns no errors", func(t *testing.T) {
		t.Parallel()

		m := &MultiError{}
		assert.Equal(t, "no errors", m.Error())
	})

	t.Run("one error delegates to first error", func(t *testing.T) {
		t.Parallel()

		be := &BindError{Field: "age", Source: SourceQuery, Value: "x", Err: assert.AnError}
		m := &MultiError{Errors: []*BindError{be}}
		assert.Contains(t, m.Error(), "age")
	})

	t.Run("multiple errors returns count message", func(t *testing.T) {
		t.Parallel()

		be1 := &BindError{Field: "a", Source: SourceQuery, Err: assert.AnError}
		be2 := &BindError{Field: "b", Source: SourceQuery, Err: assert.AnError}
		m := &MultiError{Errors: []*BindError{be1, be2}}
		assert.Contains(t, m.Error(), "2 binding errors occurred")
	})
}

// TestMultiError_Unwrap tests that Unwrap returns all errors.
func TestMultiError_Unwrap(t *testing.T) {
	t.Parallel()

	be1 := &BindError{Field: "a", Source: SourceQuery, Err: assert.AnError}
	be2 := &BindError{Field: "b", Source: SourceForm, Err: assert.AnError}
	m := &MultiError{Errors: []*BindError{be1, be2}}
	errs := m.Unwrap()
	require.Len(t, errs, 2)
	assert.ErrorIs(t, errs[0], be1)
	assert.ErrorIs(t, errs[1], be2)
}

// TestMultiError_HTTPStatus_Details_Code tests MultiError interface methods.
func TestMultiError_HTTPStatus_Details_Code(t *testing.T) {
	t.Parallel()

	be := &BindError{Field: "x", Source: SourceQuery, Err: assert.AnError}
	m := &MultiError{Errors: []*BindError{be}}
	assert.Equal(t, 400, m.HTTPStatus())
	assert.Equal(t, "multiple_binding_errors", m.Code())
	details := m.Details()
	require.NotNil(t, details)
	slice, ok := details.([]*BindError)
	require.True(t, ok)
	require.Len(t, slice, 1)
	assert.Equal(t, "x", slice[0].Field)
}

// TestMultiError_Add_HasErrors_ErrorOrNil tests Add HasErrors and ErrorOrNil.
func TestMultiError_Add_HasErrors_ErrorOrNil(t *testing.T) {
	t.Parallel()

	t.Run("empty MultiError HasErrors false ErrorOrNil nil", func(t *testing.T) {
		t.Parallel()

		m := &MultiError{}
		assert.False(t, m.HasErrors())
		assert.Nil(t, m.ErrorOrNil())
	})

	t.Run("after Add HasErrors true ErrorOrNil returns self", func(t *testing.T) {
		t.Parallel()

		m := &MultiError{}
		be := &BindError{Field: "age", Source: SourceQuery, Err: assert.AnError}
		m.Add(be)
		assert.True(t, m.HasErrors())
		err := m.ErrorOrNil()
		require.NotNil(t, err)
		var multi *MultiError
		require.ErrorAs(t, err, &multi)
		assert.Same(t, m, multi)
	})
}
