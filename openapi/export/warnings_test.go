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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWarning(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		warning  Warning
		validate func(t *testing.T, w Warning)
	}{
		{
			name: "warning with all fields",
			warning: Warning{
				Code:    DOWNLEVEL_CONST_TO_ENUM,
				Path:    "#/components/schemas/User",
				Message: "const keyword not supported in 3.0; converted to enum",
			},
			validate: func(t *testing.T, w Warning) {
				assert.Equal(t, DOWNLEVEL_CONST_TO_ENUM, w.Code)
				assert.Equal(t, "#/components/schemas/User", w.Path)
				assert.Equal(t, "const keyword not supported in 3.0; converted to enum", w.Message)
			},
		},
		{
			name: "warning with empty path",
			warning: Warning{
				Code:    DOWNLEVEL_WEBHOOKS,
				Path:    "",
				Message: "webhooks are 3.1-only; dropped",
			},
			validate: func(t *testing.T, w Warning) {
				assert.Equal(t, DOWNLEVEL_WEBHOOKS, w.Code)
				assert.Empty(t, w.Path)
				assert.Equal(t, "webhooks are 3.1-only; dropped", w.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.validate != nil {
				tt.validate(t, tt.warning)
			}
		})
	}
}

func TestWarningConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code string
	}{
		{
			name: "DOWNLEVEL_CONST_TO_ENUM",
			code: DOWNLEVEL_CONST_TO_ENUM,
		},
		{
			name: "DOWNLEVEL_CONST_TO_ENUM_CONFLICT",
			code: DOWNLEVEL_CONST_TO_ENUM_CONFLICT,
		},
		{
			name: "DOWNLEVEL_UNEVALUATED_PROPERTIES",
			code: DOWNLEVEL_UNEVALUATED_PROPERTIES,
		},
		{
			name: "DOWNLEVEL_PATTERN_PROPERTIES",
			code: DOWNLEVEL_PATTERN_PROPERTIES,
		},
		{
			name: "DOWNLEVEL_MULTIPLE_EXAMPLES",
			code: DOWNLEVEL_MULTIPLE_EXAMPLES,
		},
		{
			name: "DOWNLEVEL_WEBHOOKS",
			code: DOWNLEVEL_WEBHOOKS,
		},
		{
			name: "DOWNLEVEL_LICENSE_IDENTIFIER",
			code: DOWNLEVEL_LICENSE_IDENTIFIER,
		},
		{
			name: "DOWNLEVEL_INFO_SUMMARY",
			code: DOWNLEVEL_INFO_SUMMARY,
		},
		{
			name: "DOWNLEVEL_MUTUAL_TLS",
			code: DOWNLEVEL_MUTUAL_TLS,
		},
		{
			name: "SERVER_VARIABLE_EMPTY_ENUM",
			code: SERVER_VARIABLE_EMPTY_ENUM,
		},
		{
			name: "SERVER_VARIABLE_DEFAULT_NOT_IN_ENUM",
			code: SERVER_VARIABLE_DEFAULT_NOT_IN_ENUM,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.code, "warning code should not be empty")
		})
	}
}

func TestWarning_String(t *testing.T) {
	t.Parallel()

	w := Warning{
		Code:    DOWNLEVEL_CONST_TO_ENUM,
		Path:    "#/components/schemas/User",
		Message: "const keyword not supported in 3.0; converted to enum",
	}

	// Warning doesn't have a String() method, but we can verify the fields are accessible
	assert.Equal(t, DOWNLEVEL_CONST_TO_ENUM, w.Code)
	assert.Equal(t, "#/components/schemas/User", w.Path)
	assert.Equal(t, "const keyword not supported in 3.0; converted to enum", w.Message)
}
