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

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/binding"
	"rivaas.dev/validation"
)

func TestWithStrict(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("POST", "/test", map[string]any{
		"name":    "Alice",
		"email":   "alice@example.com",
		"age":     30,
		"unknown": "field",
	})
	require.NoError(t, err)

	var req testBindRequest
	err = c.Bind(&req, WithStrict())
	assert.Error(t, err, "should reject unknown fields")
}

func TestWithPartial(t *testing.T) {
	t.Parallel()

	type updateRequest struct {
		Name *string `json:"name" validate:"omitempty,min=2"`
	}

	c, err := TestContextWithBody("PATCH", "/test", map[string]any{})
	require.NoError(t, err)

	var req updateRequest
	err = c.Bind(&req, WithPartial())
	assert.NoError(t, err, "empty body should be valid in partial mode")
}

func TestWithoutValidation(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("POST", "/test", map[string]any{
		"name":  "A", // Too short
		"email": "not-an-email",
		"age":   -1, // Below minimum
	})
	require.NoError(t, err)

	var req testBindRequest
	err = c.Bind(&req, WithoutValidation())
	assert.NoError(t, err, "validation should be skipped")
	assert.Equal(t, "A", req.Name)
}

func TestWithPresence(t *testing.T) {
	t.Parallel()

	pm := validation.PresenceMap{
		"name": true,
	}

	c, err := TestContextWithBody("POST", "/test", map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	})
	require.NoError(t, err)

	var req testBindRequest
	err = c.Bind(&req, WithPresence(pm))
	assert.NoError(t, err)
}

func TestWithBindingOptions(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("POST", "/test", map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	})
	require.NoError(t, err)

	var req testBindRequest
	err = c.Bind(&req, WithBindingOptions(
		binding.WithMaxDepth(10),
	))
	assert.NoError(t, err)
}

func TestWithValidationOptions(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("POST", "/test", map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	})
	require.NoError(t, err)

	var req testBindRequest
	err = c.Bind(&req, WithValidationOptions(
		validation.WithMaxErrors(5),
	))
	assert.NoError(t, err)
}

func TestOptionCombinations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    any
		opts    []BindOption
		wantErr bool
	}{
		{
			name: "strict and partial together",
			body: map[string]any{
				"name": "Alice",
			},
			opts: []BindOption{
				WithStrict(),
				WithPartial(),
			},
			wantErr: false,
		},
		{
			name: "strict rejects unknown fields even with partial",
			body: map[string]any{
				"name":    "Alice",
				"unknown": "field",
			},
			opts: []BindOption{
				WithStrict(),
				WithPartial(),
			},
			wantErr: true,
		},
		{
			name: "without validation allows invalid data",
			body: map[string]any{
				"name":  "A",
				"email": "not-an-email",
				"age":   -1,
			},
			opts: []BindOption{
				WithoutValidation(),
			},
			wantErr: false,
		},
		{
			name: "multiple binding and validation options",
			body: map[string]any{
				"name":  "Alice",
				"email": "alice@example.com",
				"age":   30,
			},
			opts: []BindOption{
				WithBindingOptions(binding.WithMaxDepth(10)),
				WithValidationOptions(validation.WithMaxErrors(5)),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", tt.body)
			require.NoError(t, err)

			var req testBindRequest
			err = c.Bind(&req, tt.opts...)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApplyBindOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		opts          []BindOption
		wantStrict    bool
		wantSkipValid bool
		wantPartial   bool
	}{
		{
			name:          "no options",
			opts:          nil,
			wantStrict:    false,
			wantSkipValid: false,
			wantPartial:   false,
		},
		{
			name:          "with strict",
			opts:          []BindOption{WithStrict()},
			wantStrict:    true,
			wantSkipValid: false,
			wantPartial:   false,
		},
		{
			name:          "with partial",
			opts:          []BindOption{WithPartial()},
			wantStrict:    false,
			wantSkipValid: false,
			wantPartial:   true,
		},
		{
			name:          "without validation",
			opts:          []BindOption{WithoutValidation()},
			wantStrict:    false,
			wantSkipValid: true,
			wantPartial:   false,
		},
		{
			name: "multiple options",
			opts: []BindOption{
				WithStrict(),
				WithPartial(),
			},
			wantStrict:    true,
			wantSkipValid: false,
			wantPartial:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := applyBindOptions(tt.opts)

			assert.Equal(t, tt.wantStrict, cfg.strict)
			assert.Equal(t, tt.wantSkipValid, cfg.skipValidation)
			assert.Equal(t, tt.wantPartial, cfg.partial)
		})
	}
}
