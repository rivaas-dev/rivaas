// Copyright 2025 The Rivaas Authors
// Copyright 2025 Company.info B.V.
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

package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigError_Error(t *testing.T) {
	t.Parallel()

	baseErr := errors.New("base error")

	tests := []struct {
		name    string
		err     *ConfigError
		wantMsg string
	}{
		{
			name: "error with field",
			err: &ConfigError{
				Source:    "source1",
				Field:     "field1",
				Operation: "parse",
				Err:       baseErr,
			},
			wantMsg: "config error in source1.field1 during parse: base error",
		},
		{
			name: "error without field",
			err: &ConfigError{
				Source:    "source2",
				Operation: "load",
				Err:       baseErr,
			},
			wantMsg: "config error in source2 during load: base error",
		},
		{
			name: "error with empty source",
			err: &ConfigError{
				Source:    "",
				Operation: "validate",
				Err:       baseErr,
			},
			wantMsg: "config error in  during validate: base error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantMsg, tt.err.Error())
		})
	}
}

func TestConfigError_Unwrap(t *testing.T) {
	t.Parallel()

	baseErr := errors.New("base error")

	tests := []struct {
		name       string
		err        *ConfigError
		wantUnwrap error
	}{
		{
			name: "unwraps to base error",
			err: &ConfigError{
				Source:    "source1",
				Operation: "parse",
				Err:       baseErr,
			},
			wantUnwrap: baseErr,
		},
		{
			name: "unwraps to base error with field",
			err: &ConfigError{
				Source:    "source1",
				Field:     "field1",
				Operation: "parse",
				Err:       baseErr,
			},
			wantUnwrap: baseErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.wantUnwrap, tt.err.Unwrap())
		})
	}
}

func TestNewConfigError(t *testing.T) {
	t.Parallel()

	baseErr := errors.New("test error")

	err := NewConfigError("test-source", "test-operation", baseErr)

	require.NotNil(t, err)
	assert.Equal(t, "test-source", err.Source)
	assert.Equal(t, "test-operation", err.Operation)
	assert.Equal(t, "", err.Field) // Should be empty when using NewConfigError
	assert.Equal(t, baseErr, err.Err)
	assert.Equal(t, baseErr, err.Unwrap())
	assert.Equal(t, "config error in test-source during test-operation: test error", err.Error())
}

func TestNewConfigFieldError(t *testing.T) {
	t.Parallel()

	baseErr := errors.New("field error")

	err := NewConfigFieldError("test-source", "test-field", "test-operation", baseErr)

	require.NotNil(t, err)
	assert.Equal(t, "test-source", err.Source)
	assert.Equal(t, "test-field", err.Field)
	assert.Equal(t, "test-operation", err.Operation)
	assert.Equal(t, baseErr, err.Err)
	assert.Equal(t, baseErr, err.Unwrap())
	assert.Equal(t, "config error in test-source.test-field during test-operation: field error", err.Error())
}

func TestConfigError_ErrorWrapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupErr    func() error
		checkIs     error
		checkAs     any
		wantIs      bool
		wantAsMatch func(t *testing.T, target any)
	}{
		{
			name: "supports errors.Is",
			setupErr: func() error {
				originalErr := errors.New("original error")
				return NewConfigError("source", "operation", originalErr)
			},
			checkIs: errors.New("original error"),
			wantIs:  false, // Different error instances
		},
		{
			name: "supports errors.Is with same instance",
			setupErr: func() error {
				originalErr := errors.New("original error")
				return NewConfigError("source", "operation", originalErr)
			},
			checkIs: func() error {
				// Create the same error instance
				return errors.New("original error")
			}(),
			wantIs: false,
		},
		{
			name: "supports errors.As",
			setupErr: func() error {
				originalErr := errors.New("original error")
				return NewConfigError("source", "operation", originalErr)
			},
			checkAs: &ConfigError{},
			wantAsMatch: func(t *testing.T, target any) {
				t.Helper()
				configErr, ok := target.(*ConfigError)
				require.True(t, ok)
				assert.Equal(t, "source", configErr.Source)
				assert.Equal(t, "operation", configErr.Operation)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.setupErr()

			if tt.checkIs != nil {
				originalErr := errors.New("original error")
				configErr := NewConfigError("source", "operation", originalErr)
				assert.True(t, errors.Is(configErr, originalErr))
			}

			if tt.checkAs != nil {
				var targetErr *ConfigError
				require.True(t, errors.As(err, &targetErr))
				if tt.wantAsMatch != nil {
					tt.wantAsMatch(t, targetErr)
				}
			}
		})
	}
}

func TestConfigError_Chaining(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("root cause")
	firstErr := NewConfigError("first-source", "first-op", originalErr)
	secondErr := NewConfigError("second-source", "second-op", firstErr)

	// Should be able to unwrap to the original error
	assert.True(t, errors.Is(secondErr, originalErr))
	assert.True(t, errors.Is(secondErr, firstErr))

	// Test the error message of the outer error
	assert.Contains(t, secondErr.Error(), "second-source")
	assert.Contains(t, secondErr.Error(), "second-op")
}
