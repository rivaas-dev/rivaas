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

package validate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		spec     []byte
		version  Version
		wantErr  bool
		contains string
	}{
		{
			name:     "invalid JSON fails",
			spec:     []byte(`{invalid json`),
			version:  V30,
			wantErr:  true,
			contains: "",
		},
		{
			name:     "unsupported version fails",
			spec:     []byte(`{"openapi":"3.0.4"}`),
			version:  Version("9.9"),
			wantErr:  true,
			contains: "unsupported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			validator := New()
			ctx := context.Background()

			err := validator.Validate(ctx, tt.spec, tt.version)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.contains != "" {
					assert.ErrorContains(t, err, tt.contains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestValidator_Validate_caching(t *testing.T) {
	t.Parallel()

	// Call Validate twice with unsupported version; both should return error.
	// Exercises that getOrCompile is used (second call hits same error path).
	validator := New()
	ctx := context.Background()
	spec := []byte(`{}`)

	err1 := validator.Validate(ctx, spec, Version("9.9"))
	require.Error(t, err1)
	assert.ErrorContains(t, err1, "unsupported")

	err2 := validator.Validate(ctx, spec, Version("9.9"))
	require.Error(t, err2)
	assert.ErrorContains(t, err2, "unsupported")
}

func TestValidateResponseCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		code     string
		wantErr  bool
		contains string
	}{
		{
			name:    "default",
			code:    "default",
			wantErr: false,
		},
		{
			name:    "200",
			code:    "200",
			wantErr: false,
		},
		{
			name:    "404",
			code:    "404",
			wantErr: false,
		},
		{
			name:    "4XX",
			code:    "4XX",
			wantErr: false,
		},
		{
			name:    "5XX",
			code:    "5XX",
			wantErr: false,
		},
		{
			name:     "0 is invalid",
			code:     "0",
			wantErr:  true,
			contains: "invalid response code",
		},
		{
			name:     "600 is invalid",
			code:     "600",
			wantErr:  true,
			contains: "invalid response code",
		},
		{
			name:     "foo is invalid",
			code:     "foo",
			wantErr:  true,
			contains: "invalid response code",
		},
		{
			name:     "empty string is invalid",
			code:     "",
			wantErr:  true,
			contains: "invalid response code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateResponseCode(tt.code)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.contains != "" {
					assert.ErrorContains(t, err, tt.contains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestValidateComponentName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantErr  bool
		contains string
	}{
		{
			name:    "alphanumeric valid",
			input:   "User",
			wantErr: false,
		},
		{
			name:    "hyphen and underscore valid",
			input:   "user-name_v2",
			wantErr: false,
		},
		{
			name:    "dot valid",
			input:   "com.example.User",
			wantErr: false,
		},
		{
			name:     "empty returns error",
			input:    "",
			wantErr:  true,
			contains: "cannot be empty",
		},
		{
			name:     "invalid character returns error",
			input:    "user name",
			wantErr:  true,
			contains: "invalid component name",
		},
		{
			name:     "slash invalid",
			input:    "user/name",
			wantErr:  true,
			contains: "invalid component name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateComponentName(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.contains != "" {
					assert.ErrorContains(t, err, tt.contains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}
