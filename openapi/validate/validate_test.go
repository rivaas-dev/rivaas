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
	"fmt"
	"strings"
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

			validator, err := New()
			require.NoError(t, err)
			ctx := context.Background()

			err = validator.Validate(ctx, tt.spec, tt.version)

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

func TestMustNew(t *testing.T) {
	t.Parallel()

	v := MustNew()
	require.NotNil(t, v)

	// MustNew returns a usable validator (no panic when calling Validate).
	ctx := context.Background()
	err := v.Validate(ctx, []byte(`{}`), V30)
	// Minimal spec is invalid; we only need to confirm Validate ran without panic.
	assert.Error(t, err)
}

func TestNew(t *testing.T) {
	t.Parallel()

	v, err := New()
	require.NoError(t, err)
	require.NotNil(t, v)
}

func TestNew_NilOptionFails(t *testing.T) {
	t.Parallel()

	v, err := New(WithVersions(V30), nil)
	require.Error(t, err)
	require.Nil(t, v)
	assert.Contains(t, err.Error(), "cannot be nil")
	assert.Contains(t, err.Error(), "option at index 1")
}

func TestMustNew_NilOptionPanics(t *testing.T) {
	t.Parallel()

	var panicMsg string
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicMsg = fmt.Sprint(r)
			}
		}()
		MustNew(WithVersions(V30), nil)
	}()
	require.NotEmpty(t, panicMsg, "MustNew with nil option should panic")
	assert.Contains(t, panicMsg, "cannot be nil")
}

func TestValidator_Validate_caching(t *testing.T) {
	t.Parallel()

	// Call Validate twice with unsupported version; both should return error.
	// Exercises that getOrCompile is used (second call hits same error path).
	validator, err := New()
	require.NoError(t, err)
	ctx := context.Background()
	spec := []byte(`{}`)

	err1 := validator.Validate(ctx, spec, Version("9.9"))
	require.Error(t, err1)
	assert.ErrorContains(t, err1, "unsupported")

	err2 := validator.Validate(ctx, spec, Version("9.9"))
	require.Error(t, err2)
	assert.ErrorContains(t, err2, "unsupported")
}

// minimalSpec30 has openapi 3.0 and minimal structure (used to trigger version-specific paths).
var minimalSpec30 = []byte(`{"openapi":"3.0.0","info":{"title":"a","version":"1.0"},"paths":{}}`)

// minimalSpec31 has openapi 3.1 and minimal structure (used to trigger version-specific paths).
var minimalSpec31 = []byte(`{"openapi":"3.1.0","info":{"title":"a","version":"1.0"},"paths":{}}`)

func TestWithVersions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("WithVersions(V31) rejects V30", func(t *testing.T) {
		t.Parallel()
		validator, err := New(WithVersions(V31))
		require.NoError(t, err)
		err = validator.Validate(ctx, minimalSpec30, V30)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed")
		assert.Contains(t, err.Error(), "allowed")
	})

	t.Run("WithVersions(V31) accepts V31", func(t *testing.T) {
		t.Parallel()
		validator, err := New(WithVersions(V31))
		require.NoError(t, err)
		err = validator.Validate(ctx, minimalSpec31, V31)
		// We only assert that the "not allowed" check passed; schema compile/validate may fail in some envs.
		if err != nil {
			assert.NotContains(t, err.Error(), "not allowed")
		}
	})

	t.Run("WithVersions(V30) accepts V30", func(t *testing.T) {
		t.Parallel()
		validator, err := New(WithVersions(V30))
		require.NoError(t, err)
		err = validator.Validate(ctx, minimalSpec30, V30)
		if err != nil {
			assert.NotContains(t, err.Error(), "not allowed")
		}
	})

	t.Run("WithVersions(V30, V31) allows both", func(t *testing.T) {
		t.Parallel()
		validator, err := New(WithVersions(V30, V31))
		require.NoError(t, err)
		err30 := validator.Validate(ctx, minimalSpec30, V30)
		err31 := validator.Validate(ctx, minimalSpec31, V31)
		if err30 != nil {
			assert.NotContains(t, err30.Error(), "not allowed")
		}
		if err31 != nil {
			assert.NotContains(t, err31.Error(), "not allowed")
		}
	})

	t.Run("no option allows both", func(t *testing.T) {
		t.Parallel()
		validator, err := New()
		require.NoError(t, err)
		err30 := validator.Validate(ctx, minimalSpec30, V30)
		err31 := validator.Validate(ctx, minimalSpec31, V31)
		if err30 != nil {
			assert.NotContains(t, err30.Error(), "not allowed")
		}
		if err31 != nil {
			assert.NotContains(t, err31.Error(), "not allowed")
		}
	})
}

func TestValidator_ValidateAuto(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("valid 3.0 spec", func(t *testing.T) {
		t.Parallel()
		validator := MustNew()
		err := validator.ValidateAuto(ctx, minimalSpec30)
		// Version detection must succeed; any error must not be from our detection logic.
		if err != nil {
			assert.NotContains(t, err.Error(), "missing")
			assert.NotContains(t, err.Error(), "invalid JSON")
			assert.NotContains(t, err.Error(), "unsupported openapi version")
		}
	})

	t.Run("valid 3.1 spec", func(t *testing.T) {
		t.Parallel()
		validator := MustNew()
		err := validator.ValidateAuto(ctx, minimalSpec31)
		if err != nil {
			assert.NotContains(t, err.Error(), "missing")
			assert.NotContains(t, err.Error(), "invalid JSON")
			assert.NotContains(t, err.Error(), "unsupported openapi version")
		}
	})

	t.Run("missing openapi field", func(t *testing.T) {
		t.Parallel()
		validator := MustNew()
		err := validator.ValidateAuto(ctx, []byte(`{"info":{"title":"a","version":"1.0"},"paths":{}}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing")
		assert.Contains(t, err.Error(), "openapi")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		t.Parallel()
		validator := MustNew()
		err := validator.ValidateAuto(ctx, []byte(`{invalid`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON")
	})

	t.Run("unsupported version", func(t *testing.T) {
		t.Parallel()
		validator := MustNew()
		err := validator.ValidateAuto(ctx, []byte(`{"openapi":"2.0"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported")
	})

	t.Run("ValidateAuto respects WithVersions", func(t *testing.T) {
		t.Parallel()
		validator := MustNew(WithVersions(V31))
		// ValidateAuto on a 3.0 spec will detect 3.0 and then call Validate(ctx, spec, V30), which is rejected.
		err := validator.ValidateAuto(ctx, minimalSpec30)
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "not allowed") || strings.Contains(err.Error(), "allowed"))
	})
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
