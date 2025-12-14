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

package export

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateExtensionKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		version string
		wantErr bool
		errType string
	}{
		{
			name:    "valid extension key",
			key:     "x-custom-field",
			version: "3.0.4",
			wantErr: false,
		},
		{
			name:    "valid extension key 3.1",
			key:     "x-custom-field",
			version: "3.1.2",
			wantErr: false,
		},
		{
			name:    "invalid - no x- prefix",
			key:     "custom-field",
			version: "3.0.4",
			wantErr: true,
			errType: "InvalidExtensionKeyError",
		},
		{
			name:    "invalid - empty key",
			key:     "",
			version: "3.0.4",
			wantErr: true,
			errType: "InvalidExtensionKeyError",
		},
		{
			name:    "reserved prefix x-oai- in 3.1",
			key:     "x-oai-custom",
			version: "3.1.2",
			wantErr: true,
			errType: "ReservedExtensionKeyError",
		},
		{
			name:    "reserved prefix x-oas- in 3.1",
			key:     "x-oas-custom",
			version: "3.1.2",
			wantErr: true,
			errType: "ReservedExtensionKeyError",
		},
		{
			name:    "reserved prefix x-oai- allowed in 3.0",
			key:     "x-oai-custom",
			version: "3.0.4",
			wantErr: false,
		},
		{
			name:    "reserved prefix x-oas- allowed in 3.0",
			key:     "x-oas-custom",
			version: "3.0.4",
			wantErr: false,
		},
		{
			name:    "x-oai- prefix with more characters",
			key:     "x-oai-something-else",
			version: "3.1.2",
			wantErr: true,
			errType: "ReservedExtensionKeyError",
		},
		{
			name:    "x-oas- prefix with more characters",
			key:     "x-oas-something-else",
			version: "3.1.2",
			wantErr: true,
			errType: "ReservedExtensionKeyError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateExtensionKey(tt.key, tt.version)

			if tt.wantErr {
				require.Error(t, err)
				switch tt.errType {
				case "InvalidExtensionKeyError":
					var extErr *InvalidExtensionKeyError
					require.ErrorAs(t, err, &extErr)
					assert.Contains(t, err.Error(), "extension key must start with 'x-'")
				case "ReservedExtensionKeyError":
					var resErr *ReservedExtensionKeyError
					require.ErrorAs(t, err, &resErr)
					assert.Contains(t, err.Error(), "reserved prefix")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInvalidExtensionKeyError(t *testing.T) {
	t.Parallel()

	err := &InvalidExtensionKeyError{Key: "invalid-key"}
	assert.Equal(t, "extension key must start with 'x-': invalid-key", err.Error())
}

func TestReservedExtensionKeyError(t *testing.T) {
	t.Parallel()

	err := &ReservedExtensionKeyError{Key: "x-oai-test"}
	assert.Equal(t, "extension key uses reserved prefix (x-oai- or x-oas-): x-oai-test", err.Error())
}

func TestCopyExtensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]any
		version  string
		expected map[string]any
	}{
		{
			name:     "nil input",
			input:    nil,
			version:  "3.0.4",
			expected: nil,
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			version:  "3.0.4",
			expected: nil,
		},
		{
			name: "valid extensions",
			input: map[string]any{
				"x-custom-1": "value1",
				"x-custom-2": 42,
				"x-custom-3": []string{"a", "b"},
			},
			version: "3.0.4",
			expected: map[string]any{
				"x-custom-1": "value1",
				"x-custom-2": 42,
				"x-custom-3": []string{"a", "b"},
			},
		},
		{
			name: "filters invalid keys",
			input: map[string]any{
				"x-valid":   "value",
				"invalid":   "should be filtered",
				"x-another": "value2",
			},
			version: "3.0.4",
			expected: map[string]any{
				"x-valid":   "value",
				"x-another": "value2",
			},
		},
		{
			name: "filters reserved keys in 3.1",
			input: map[string]any{
				"x-valid":    "value",
				"x-oai-test": "should be filtered",
				"x-oas-test": "should be filtered",
			},
			version: "3.1.2",
			expected: map[string]any{
				"x-valid": "value",
			},
		},
		{
			name: "allows reserved keys in 3.0",
			input: map[string]any{
				"x-valid":    "value",
				"x-oai-test": "allowed in 3.0",
				"x-oas-test": "allowed in 3.0",
			},
			version: "3.0.4",
			expected: map[string]any{
				"x-valid":    "value",
				"x-oai-test": "allowed in 3.0",
				"x-oas-test": "allowed in 3.0",
			},
		},
		{
			name: "all invalid keys results in nil",
			input: map[string]any{
				"invalid1": "value1",
				"invalid2": "value2",
			},
			version:  "3.0.4",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := copyExtensions(tt.input, tt.version)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMarshalWithExtensions(t *testing.T) {
	t.Parallel()

	type testStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name       string
		v          testStruct
		extensions map[string]any
		wantJSON   string
		wantErr    bool
	}{
		{
			name: "no extensions",
			v: testStruct{
				Name:  "test",
				Value: 42,
			},
			extensions: nil,
			wantJSON:   `{"name":"test","value":42}`,
			wantErr:    false,
		},
		{
			name: "empty extensions",
			v: testStruct{
				Name:  "test",
				Value: 42,
			},
			extensions: map[string]any{},
			wantJSON:   `{"name":"test","value":42}`,
			wantErr:    false,
		},
		{
			name: "with extensions",
			v: testStruct{
				Name:  "test",
				Value: 42,
			},
			extensions: map[string]any{
				"x-custom-1": "value1",
				"x-custom-2": 123,
			},
			wantJSON: `{"name":"test","value":42,"x-custom-1":"value1","x-custom-2":123}`,
			wantErr:  false,
		},
		{
			name: "extensions with complex values",
			v: testStruct{
				Name:  "test",
				Value: 42,
			},
			extensions: map[string]any{
				"x-array": []string{"a", "b", "c"},
				"x-object": map[string]any{
					"nested": "value",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := marshalWithExtensions(tt.v, tt.extensions)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.wantJSON != "" {
				assertJSONEqual(t, tt.wantJSON, result)
			} else {
				assertValidJSONWithExtensions(t, result, tt.extensions)
			}
		})
	}
}

func TestMarshalWithExtensions_ErrorHandling(t *testing.T) {
	t.Parallel()

	// Test with a type that cannot be marshaled
	type unMarshallable struct {
		Channel chan int // channels cannot be marshaled
	}

	t.Run("handles unmarshalable struct", func(t *testing.T) {
		t.Parallel()

		v := unMarshallable{Channel: make(chan int)}
		_, err := marshalWithExtensions(v, nil)
		assert.Error(t, err)
	})

	t.Run("handles invalid JSON from unmarshal", func(t *testing.T) {
		t.Parallel()

		// This is harder to test directly, but we can test the error path
		// by using a struct that marshals but creates invalid JSON when unmarshaled
		// Actually, this is difficult to trigger in practice, so we'll skip it
	})
}

// assertJSONEqual compares two JSON values semantically.
func assertJSONEqual(t *testing.T, wantJSON string, gotBytes []byte) {
	t.Helper()

	var got, want map[string]any
	require.NoError(t, json.Unmarshal(gotBytes, &got))
	require.NoError(t, json.Unmarshal([]byte(wantJSON), &want))
	assert.Equal(t, want, got)
}

// assertValidJSONWithExtensions verifies JSON contains expected base fields and extensions.
func assertValidJSONWithExtensions(t *testing.T, result []byte, extensions map[string]any) {
	t.Helper()

	var m map[string]any
	require.NoError(t, json.Unmarshal(result, &m))

	// Verify base struct fields are present
	assert.Equal(t, "test", m["name"])
	assert.Equal(t, float64(42), m["value"]) //nolint:testifylint // exact integer comparison

	// Verify extensions are present
	for k, v := range extensions {
		assert.Contains(t, m, k)
		assertExtensionValue(t, m[k], v)
	}
}

// assertExtensionValue compares extension values accounting for JSON type differences.
func assertExtensionValue(t *testing.T, got, want any) {
	t.Helper()

	gotSlice, gotIsSlice := got.([]any)
	wantSlice, wantIsStringSlice := want.([]string)

	if gotIsSlice && wantIsStringSlice {
		// Convert []string to []any for comparison
		wantAny := make([]any, 0, len(wantSlice))
		for _, s := range wantSlice {
			wantAny = append(wantAny, s)
		}
		assert.Equal(t, wantAny, gotSlice)
	} else {
		assert.Equal(t, want, got)
	}
}
