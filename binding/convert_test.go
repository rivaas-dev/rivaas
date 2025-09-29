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

package binding

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertValue_ErrorCases tests convertValue error paths
func TestConvertValue_ErrorCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		value          string
		kind           reflect.Kind
		expectedErrMsg string
	}{
		// Invalid unsigned integer errors
		{"negative value for uint", "-42", reflect.Uint, "invalid unsigned integer"},
		{"negative value for uint8", "-10", reflect.Uint8, "invalid unsigned integer"},
		{"negative value for uint16", "-100", reflect.Uint16, "invalid unsigned integer"},
		{"negative value for uint32", "-1000", reflect.Uint32, "invalid unsigned integer"},
		{"negative value for uint64", "-999999", reflect.Uint64, "invalid unsigned integer"},
		{"invalid format for uint", "abc", reflect.Uint, "invalid unsigned integer"},
		{"decimal for uint", "42.5", reflect.Uint, "invalid unsigned integer"},
		{"empty string for uint", "", reflect.Uint, "invalid unsigned integer"},
		{"whitespace for uint", "   ", reflect.Uint, "invalid unsigned integer"},
		// Invalid float errors
		{"invalid format for float32", "abc", reflect.Float32, "invalid float"},
		{"invalid format for float64", "xyz", reflect.Float64, "invalid float"},
		{"empty string for float32", "", reflect.Float32, "invalid float"},
		{"whitespace for float64", "   ", reflect.Float64, "invalid float"},
		{"mixed chars for float32", "12abc", reflect.Float32, "invalid float"},
		{"only dots for float64", "...", reflect.Float64, "invalid float"},
		{"multiple dots for float32", "12.34.56", reflect.Float32, "invalid float"},
		// Invalid bool errors
		{"invalid bool value", "maybe", reflect.Bool, "invalid"},
		{"numeric 2", "2", reflect.Bool, "invalid"},
		{"numeric 3", "3", reflect.Bool, "invalid"},
		{"random text", "random", reflect.Bool, "invalid"},
		{"mixed case invalid", "Maybe", reflect.Bool, "invalid"},
		{"yesno together", "yesno", reflect.Bool, "invalid"},
		{"truefalse together", "truefalse", reflect.Bool, "invalid"},
		// Unsupported type errors
		{"slice type", "", reflect.Slice, "unsupported type"},
		{"map type", "", reflect.Map, "unsupported type"},
		{"array type", "", reflect.Array, "unsupported type"},
		{"chan type", "", reflect.Chan, "unsupported type"},
		{"func type", "", reflect.Func, "unsupported type"},
		{"interface type", "", reflect.Interface, "unsupported type"},
		{"ptr type", "", reflect.Ptr, "unsupported type"},
		{"struct type", "", reflect.Struct, "unsupported type"},
		{"unsafe pointer", "", reflect.UnsafePointer, "unsupported type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := defaultOptions()
			_, err := convertValue(tt.value, tt.kind, opts)

			require.Error(t, err, "Expected error for %s", tt.name)
			assert.ErrorContains(t, err, tt.expectedErrMsg, "Error should contain %q", tt.expectedErrMsg)
		})
	}
}

// TestExtractBracketKey_EdgeCases tests extractBracketKey edge cases
func TestExtractBracketKey_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		prefix   string
		expected string
	}{
		{"no prefix match", "other[key]", "prefix", ""},
		{"no closing bracket", "prefix[unclosed", "prefix", ""},
		{"empty brackets", "prefix[]", "prefix", ""},
		{"nested brackets", "prefix[key1][key2]", "prefix", ""},
		{"quoted key with double quotes", `prefix["key.with.dots"]`, "prefix", "key.with.dots"},
		{"quoted key with single quotes", "prefix['key-with-dash']", "prefix", "key-with-dash"},
		{"quoted key empty after trimming", `prefix[""]`, "prefix", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := extractBracketKey(tt.input, tt.prefix)
			assert.Equal(t, tt.expected, result, "extractBracketKey(%q, %q) = %q, want %q", tt.input, tt.prefix, result, tt.expected)
		})
	}
}
