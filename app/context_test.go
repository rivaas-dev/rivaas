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

	"rivaas.dev/validation"
)

func TestContext_Bind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    any
		wantErr bool
	}{
		{
			name: "valid request",
			body: map[string]any{
				"name":  "Alice",
				"email": "alice@example.com",
				"age":   30,
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			body: map[string]any{
				"name": "Alice",
			},
			wantErr: true,
		},
		{
			name: "invalid email",
			body: map[string]any{
				"name":  "Alice",
				"email": "not-an-email",
				"age":   30,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", tt.body)
			require.NoError(t, err)

			var req testBindRequest
			err = c.Bind(&req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				body, ok := tt.body.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, body["name"], req.Name)
			}
		})
	}
}

func TestContext_Bind_WithOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    any
		opts    []BindOption
		wantErr bool
	}{
		{
			name: "strict mode rejects unknown fields",
			body: map[string]any{
				"name":    "Alice",
				"email":   "alice@example.com",
				"age":     30,
				"unknown": "field",
			},
			opts:    []BindOption{WithStrict()},
			wantErr: true,
		},
		{
			name: "partial mode allows missing required fields",
			body: map[string]any{
				"name": "Alice",
			},
			opts:    []BindOption{WithPartial()},
			wantErr: false,
		},
		{
			name: "without validation skips validation",
			body: map[string]any{
				"name":  "A",
				"email": "not-an-email",
			},
			opts:    []BindOption{WithoutValidation()},
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

func TestContext_MustBind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		body   any
		wantOK bool
	}{
		{
			name: "valid request returns true",
			body: map[string]any{
				"name":  "Alice",
				"email": "alice@example.com",
				"age":   30,
			},
			wantOK: true,
		},
		{
			name: "invalid request returns false",
			body: map[string]any{
				"name": "Alice",
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", tt.body)
			require.NoError(t, err)

			var req testBindRequest
			ok := c.MustBind(&req)

			assert.Equal(t, tt.wantOK, ok)
			if ok {
				body, bodyOK := tt.body.(map[string]any)
				require.True(t, bodyOK)
				assert.Equal(t, body["name"], req.Name)
			}
		})
	}
}

func TestContext_BindOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    any
		wantErr bool
	}{
		{
			name: "binds without validation",
			body: map[string]any{
				"name":  "A", // Too short but validation skipped
				"email": "not-an-email",
				"age":   -1,
			},
			wantErr: false,
		},
		{
			name:    "malformed JSON fails",
			body:    "not-json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", tt.body)
			require.NoError(t, err)

			var req testBindRequest
			err = c.BindOnly(&req)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContext_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     testBindRequest
		wantErr bool
	}{
		{
			name: "valid struct passes validation",
			req: testBindRequest{
				Name:  "Alice",
				Email: "alice@example.com",
				Age:   30,
			},
			wantErr: false,
		},
		{
			name: "invalid email fails validation",
			req: testBindRequest{
				Name:  "Alice",
				Email: "not-an-email",
				Age:   30,
			},
			wantErr: true,
		},
		{
			name: "age below minimum fails",
			req: testBindRequest{
				Name:  "Alice",
				Email: "alice@example.com",
				Age:   -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", nil)
			require.NoError(t, err)

			err = c.Validate(&tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				var verr *validation.Error
				assert.ErrorAs(t, err, &verr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContext_Presence(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("POST", "/test", map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
	})
	require.NoError(t, err)

	// Bind to trigger presence tracking
	var req testBindRequest
	err = c.Bind(&req)
	require.NoError(t, err)

	pm := c.Presence()
	require.NotNil(t, pm)
	assert.True(t, pm.Has("name"))
	assert.True(t, pm.Has("email"))
	assert.False(t, pm.Has("age"))
}

func TestContext_ResetBinding(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("POST", "/test", map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	})
	require.NoError(t, err)

	// First bind
	var req1 testBindRequest
	err = c.Bind(&req1)
	require.NoError(t, err)

	// Reset binding metadata
	c.ResetBinding()

	// Presence should be nil after reset
	pm := c.Presence()
	assert.Nil(t, pm)
}
