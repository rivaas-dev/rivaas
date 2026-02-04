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
)

type testBindRequest struct {
	Name  string `json:"name" validate:"required,min=2"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0,lte=150"`
}

func TestBind(t *testing.T) {
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
				"age":  30,
			},
			wantErr: true,
		},
		{
			name: "invalid email format",
			body: map[string]any{
				"name":  "Alice",
				"email": "not-an-email",
				"age":   30,
			},
			wantErr: true,
		},
		{
			name: "age below minimum",
			body: map[string]any{
				"name":  "Alice",
				"email": "alice@example.com",
				"age":   -1,
			},
			wantErr: true,
		},
		{
			name: "name too short",
			body: map[string]any{
				"name":  "A",
				"email": "alice@example.com",
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

			req, err := Bind[testBindRequest](c)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				body, ok := tt.body.(map[string]any)
				require.True(t, ok)
				assert.Equal(t, body["name"], req.Name)
				assert.Equal(t, body["email"], req.Email)
			}
		})
	}
}

func TestBind_WithStrict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    any
		wantErr bool
	}{
		{
			name: "valid request without extra fields",
			body: map[string]any{
				"name":  "Alice",
				"email": "alice@example.com",
				"age":   30,
			},
			wantErr: false,
		},
		{
			name: "request with unknown field",
			body: map[string]any{
				"name":    "Alice",
				"email":   "alice@example.com",
				"age":     30,
				"unknown": "field",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", tt.body)
			require.NoError(t, err)

			req, err := Bind[testBindRequest](c, WithStrict())

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

func TestBind_WithPartial(t *testing.T) {
	t.Parallel()

	type updateRequest struct {
		Name  *string `json:"name" validate:"omitempty,min=2"`
		Email *string `json:"email" validate:"omitempty,email"`
	}

	tests := []struct {
		name    string
		body    any
		wantErr bool
	}{
		{
			name: "partial update with name only",
			body: map[string]any{
				"name": "Alice",
			},
			wantErr: false,
		},
		{
			name: "partial update with email only",
			body: map[string]any{
				"email": "alice@example.com",
			},
			wantErr: false,
		},
		{
			name:    "empty body allowed in partial mode",
			body:    map[string]any{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("PATCH", "/test", tt.body)
			require.NoError(t, err)

			_, err = Bind[updateRequest](c, WithPartial())

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMustBind(t *testing.T) {
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

			req, ok := MustBind[testBindRequest](c)

			assert.Equal(t, tt.wantOK, ok)
			if ok {
				body, bodyOK := tt.body.(map[string]any)
				require.True(t, bodyOK)
				assert.Equal(t, body["name"], req.Name)
			}
		})
	}
}

func TestBindOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    any
		wantErr bool
	}{
		{
			name: "valid binding without validation",
			body: map[string]any{
				"name":  "A", // Too short, but validation is skipped
				"email": "not-an-email",
				"age":   -1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", tt.body)
			require.NoError(t, err)

			req, err := BindOnly[testBindRequest](c)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Values are bound even if they would fail validation
				if bodyMap, ok := tt.body.(map[string]any); ok {
					assert.Equal(t, bodyMap["name"], req.Name)
				}
			}
		})
	}
}

func TestBindPatch(t *testing.T) {
	t.Parallel()

	type updateRequest struct {
		Name *string `json:"name" validate:"omitempty,min=2"`
	}

	c, err := TestContextWithBody("PATCH", "/test", map[string]any{
		"name": "Alice",
	})
	require.NoError(t, err)

	req, err := BindPatch[updateRequest](c)
	require.NoError(t, err)
	assert.NotNil(t, req.Name)
	assert.Equal(t, "Alice", *req.Name)
}

func TestMustBindPatch(t *testing.T) {
	t.Parallel()

	type updateRequest struct {
		Name *string `json:"name" validate:"omitempty,min=2"`
	}

	c, err := TestContextWithBody("PATCH", "/test", map[string]any{
		"name": "Alice",
	})
	require.NoError(t, err)

	req, ok := MustBindPatch[updateRequest](c)
	require.True(t, ok)
	assert.NotNil(t, req.Name)
	assert.Equal(t, "Alice", *req.Name)
}

func TestBindStrict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    any
		wantErr bool
	}{
		{
			name: "valid request without extra fields",
			body: map[string]any{
				"name":  "Alice",
				"email": "alice@example.com",
				"age":   30,
			},
			wantErr: false,
		},
		{
			name: "request with unknown field fails",
			body: map[string]any{
				"name":    "Alice",
				"email":   "alice@example.com",
				"age":     30,
				"unknown": "field",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := TestContextWithBody("POST", "/test", tt.body)
			require.NoError(t, err)

			req, err := BindStrict[testBindRequest](c)

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

func TestMustBindStrict(t *testing.T) {
	t.Parallel()

	c, err := TestContextWithBody("POST", "/test", map[string]any{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	})
	require.NoError(t, err)

	req, ok := MustBindStrict[testBindRequest](c)
	require.True(t, ok)
	assert.Equal(t, "Alice", req.Name)
}
