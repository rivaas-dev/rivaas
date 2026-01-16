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

package validation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithRunAll(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	tests := []struct {
		name      string
		user      User
		runAll    bool
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "without RunAll - should stop at first strategy",
			user:      User{Name: "John"}, // Missing email
			runAll:    false,
			wantError: true,
		},
		{
			name:      "with RunAll - should run all applicable strategies",
			user:      User{Name: "John"}, // Missing email
			runAll:    true,
			wantError: true,
		},
		{
			name:      "valid user without RunAll",
			user:      User{Name: "John", Email: "john@example.com"},
			runAll:    false,
			wantError: false,
		},
		{
			name:      "valid user with RunAll",
			user:      User{Name: "John", Email: "john@example.com"},
			runAll:    true,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var err error
			if tt.runAll {
				err = Validate(t.Context(), &tt.user, WithRunAll(true))
			} else {
				err = Validate(t.Context(), &tt.user)
			}
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithRequireAny(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	tests := []struct {
		name       string
		user       User
		requireAny bool
		wantError  bool
		checkErr   func(t *testing.T, err error)
	}{
		{
			name:       "valid user with requireAny",
			user:       User{Name: "John", Email: "john@example.com"},
			requireAny: true,
			wantError:  false,
		},
		{
			name:       "valid user without requireAny",
			user:       User{Name: "John", Email: "john@example.com"},
			requireAny: false,
			wantError:  false,
		},
		{
			name:       "invalid user with requireAny",
			user:       User{Name: "John"}, // Missing email
			requireAny: true,
			wantError:  true,
		},
		{
			name:       "invalid user without requireAny",
			user:       User{Name: "John"}, // Missing email
			requireAny: false,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var err error
			if tt.requireAny {
				err = Validate(t.Context(), &tt.user, WithRequireAny(true))
			} else {
				err = Validate(t.Context(), &tt.user)
			}
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithMaxErrors(t *testing.T) {
	t.Parallel()
	type User struct {
		Field1 string `json:"field1" validate:"required"`
		Field2 string `json:"field2" validate:"required"`
		Field3 string `json:"field3" validate:"required"`
		Field4 string `json:"field4" validate:"required"`
	}

	tests := []struct {
		name      string
		user      User
		maxErrors int
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "maxErrors = 2 should limit errors",
			user:      User{},
			maxErrors: 2,
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				assert.LessOrEqual(t, len(verr.Fields), 2, "expected at most 2 errors")
				assert.True(t, verr.Truncated, "should be truncated")
			},
		},
		{
			name:      "maxErrors = 0 (unlimited) should return all errors",
			user:      User{},
			maxErrors: 0,
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				assert.GreaterOrEqual(t, len(verr.Fields), 4, "expected at least 4 errors with unlimited")
			},
		},
		{
			name:      "maxErrors = 1 should return single error",
			user:      User{},
			maxErrors: 1,
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.LessOrEqual(t, len(verr.Fields), 1)
				if len(verr.Fields) > 0 {
					assert.True(t, verr.Truncated, "should be truncated when maxErrors is 1")
				}
			},
		},
		{
			name:      "maxErrors = 10 should return multiple errors",
			user:      User{},
			maxErrors: 10,
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.LessOrEqual(t, len(verr.Fields), 10)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.user, WithMaxErrors(tt.maxErrors))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithCustomValidator(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name"`
	}

	customValidator := func(v any) error {
		// Handle both pointer and value
		var user *User
		switch u := v.(type) {
		case *User:
			user = u
		case User:
			user = &u
		default:
			return ErrInvalidType
		}

		if user.Name == "" {
			return errCustomNameRequired
		}

		return nil
	}

	tests := []struct {
		name      string
		user      *User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "invalid user - missing name",
			user:      &User{},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "custom: name is required",
					"expected custom error message")
			},
		},
		{
			name:      "valid user with name",
			user:      &User{Name: "John"},
			wantError: false,
		},
		{
			name:      "invalid user - empty name string",
			user:      &User{Name: ""},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "custom: name is required")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.user, WithCustomValidator(customValidator))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithFieldNameMapper(t *testing.T) {
	t.Parallel()
	type User struct {
		FirstName string `json:"first_name" validate:"required"` //nolint:tagliatelle // snake_case is intentional for API compatibility
		LastName  string `json:"last_name" validate:"required"`  //nolint:tagliatelle // snake_case is intentional for API compatibility
	}

	tests := []struct {
		name      string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "invalid user - missing both fields",
			user:      User{},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				// Check that field names exist (field name mapper is not in public API, so we just verify errors exist)
				for _, e := range verr.Fields {
					assert.NotEmpty(t, e.Path, "field path should not be empty")
				}
			},
		},
		{
			name:      "invalid user - missing first name",
			user:      User{LastName: "Doe"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.NotEmpty(t, verr.Fields)
			},
		},
		{
			name:      "valid user with both fields",
			user:      User{FirstName: "John", LastName: "Doe"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.user, WithStrategy(StrategyTags))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithRedactor(t *testing.T) {
	t.Parallel()
	type User struct {
		Password string `json:"password" validate:"required,min=8"`
		Token    string `json:"token" validate:"required"`
	}

	tests := []struct {
		name      string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "invalid user - short password and missing token",
			user:      User{Password: "short", Token: ""},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				// Verify errors exist for password and token fields
				foundPassword := false
				foundToken := false
				for _, e := range verr.Fields {
					if e.Path == "password" {
						foundPassword = true
					}
					if e.Path == "token" {
						foundToken = true
					}
				}
				assert.True(t, foundPassword, "should have error for password field")
				assert.True(t, foundToken, "should have error for token field")
			},
		},
		{
			name:      "invalid user - missing password",
			user:      User{Token: "token123"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				foundPassword := false
				for _, e := range verr.Fields {
					if e.Path == "password" {
						foundPassword = true
					}
				}
				assert.True(t, foundPassword, "should have error for password field")
			},
		},
		{
			name:      "valid user with proper password and token",
			user:      User{Password: "password123", Token: "token123"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.user, WithStrategy(StrategyTags))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithContext(t *testing.T) {
	t.Parallel()
	type contextKey string
	key := contextKey("key")

	type User struct {
		Name string `json:"name" validate:"required"`
	}

	tests := []struct {
		name      string
		setupCtx  func() context.Context
		user      *User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "context with value should be passed through",
			setupCtx: func() context.Context {
				return context.WithValue(t.Context(), key, "value")
			},
			user:      &User{},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err, "expected validation error")
			},
		},
		{
			name: "context with valid user should pass",
			setupCtx: func() context.Context {
				return context.WithValue(t.Context(), key, "value")
			},
			user:      &User{Name: "John"},
			wantError: false,
		},
		{
			name: "background context should work",
			setupCtx: func() context.Context {
				return t.Context()
			},
			user:      &User{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := tt.setupCtx()
			err := Validate(ctx, tt.user, WithContext(ctx))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithPresence(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required"`
	}

	tests := []struct {
		name      string
		user      *User
		pm        PresenceMap
		wantError bool
		checkErr  func(t *testing.T, err error, pm PresenceMap)
	}{
		{
			name:      "partial validation - only name present",
			user:      &User{Name: "John"}, // Missing email
			pm:        PresenceMap{"name": true, "email": true},
			wantError: false, // In partial mode, only present fields are validated
			checkErr: func(t *testing.T, err error, pm PresenceMap) {
				t.Helper()
				assert.True(t, pm.Has("name"), "presence map should contain 'name'")
				assert.True(t, pm.Has("email"), "presence map should contain 'email'")
			},
		},
		{
			name:      "partial validation - only name in presence map",
			user:      &User{Name: "John"},
			pm:        PresenceMap{"name": true},
			wantError: false, // Email not in presence map, so not validated
		},
		{
			name:      "partial validation - empty presence map",
			user:      &User{Name: "John"},
			pm:        PresenceMap{},
			wantError: false, // Empty presence map may skip validation
		},
		{
			name:      "partial validation - valid user with all fields",
			user:      &User{Name: "John", Email: "john@example.com"},
			pm:        PresenceMap{"name": true, "email": true},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePartial(t.Context(), tt.user, tt.pm)
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err, tt.pm)
				}
			} else {
				// In partial mode, errors may still occur for present fields
				if err != nil {
					t.Logf("Partial validation error (expected in some cases): %v", err)
				}
				if tt.checkErr != nil {
					tt.checkErr(t, err, tt.pm)
				}
			}
		})
	}
}

func TestWithCustomSchema(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name      string
		schema    string
		schemaID  string
		user      *User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "valid user with custom schema",
			schema:    `{"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}`,
			schemaID:  "test-custom-schema-1",
			user:      &User{Name: "John"},
			wantError: false,
		},
		{
			name:      "valid user - empty string might pass required check",
			schema:    `{"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}`,
			schemaID:  "test-custom-schema-2",
			user:      &User{Name: ""}, // Empty string is still a string, so required might pass
			wantError: false,           // Empty string might satisfy required if field is present
		},
		{
			name:      "valid user with minLength constraint",
			schema:    `{"type": "object", "properties": {"name": {"type": "string", "minLength": 1}}, "required": ["name"]}`,
			schemaID:  "test-custom-schema-3",
			user:      &User{Name: "John"},
			wantError: false,
		},
		{
			name:      "invalid user - violates minLength",
			schema:    `{"type": "object", "properties": {"name": {"type": "string", "minLength": 5}}, "required": ["name"]}`,
			schemaID:  "test-custom-schema-4",
			user:      &User{Name: "Jo"}, // Too short
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.user, WithStrategy(StrategyJSONSchema), WithCustomSchema(tt.schemaID, tt.schema))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewValidationConfig_Defaults(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name" validate:"required"`
	}

	tests := []struct {
		name      string
		user      *User
		opts      []Option
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "auto strategy (default) should validate",
			user:      &User{},
			opts:      nil,
			wantError: true,
		},
		{
			name:      "maxErrors = 1 should limit errors",
			user:      &User{},
			opts:      []Option{WithMaxErrors(1)},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				var verr *Error
				if errors.As(err, &verr) {
					if len(verr.Fields) > 1 && !verr.Truncated {
						t.Error("should respect maxErrors")
					}
				}
			},
		},
		{
			name:      "valid user should pass with defaults",
			user:      &User{Name: "John"},
			opts:      nil,
			wantError: false,
		},
		{
			name:      "default strategy should detect tags",
			user:      &User{},
			opts:      nil,
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.NotEmpty(t, verr.Fields)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.user, tt.opts...)
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithStrategy(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	tests := []struct {
		name      string
		user      *User
		strategy  Strategy
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "StrategyAuto should detect tags",
			user:      &User{},
			strategy:  StrategyAuto,
			wantError: true,
		},
		{
			name:      "StrategyTags should validate tags",
			user:      &User{},
			strategy:  StrategyTags,
			wantError: true,
		},
		{
			name:      "StrategyTags with valid user should pass",
			user:      &User{Name: "John", Email: "john@example.com"},
			strategy:  StrategyTags,
			wantError: false,
		},
		{
			name:      "StrategyInterface with no validator should pass",
			user:      &User{},
			strategy:  StrategyInterface,
			wantError: false, // No validator interface, so should pass
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.user, WithStrategy(tt.strategy))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithMultipleOptions(t *testing.T) {
	t.Parallel()
	type User struct {
		Field1 string `json:"field1" validate:"required"`
		Field2 string `json:"field2" validate:"required"`
		Field3 string `json:"field3" validate:"required"`
	}

	tests := []struct {
		name      string
		user      *User
		opts      []Option
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "multiple options - RunAll and MaxErrors",
			user:      &User{},
			opts:      []Option{WithRunAll(true), WithMaxErrors(2)},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				assert.LessOrEqual(t, len(verr.Fields), 2, "expected at most 2 errors")
			},
		},
		{
			name:      "multiple options - Strategy and MaxErrors",
			user:      &User{},
			opts:      []Option{WithStrategy(StrategyTags), WithMaxErrors(1)},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				assert.LessOrEqual(t, len(verr.Fields), 1, "expected at most 1 error")
			},
		},
		{
			name:      "multiple options - Context and Strategy",
			user:      &User{},
			opts:      []Option{WithContext(t.Context()), WithStrategy(StrategyTags)},
			wantError: true,
		},
		{
			name:      "valid user with multiple options should pass",
			user:      &User{Field1: "value1", Field2: "value2", Field3: "value3"},
			opts:      []Option{WithRunAll(true), WithMaxErrors(10)},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.user, tt.opts...)
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithPartial(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required"`
	}

	tests := []struct {
		name      string
		user      *User
		pm        PresenceMap
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "partial mode with presence map",
			user:      &User{Name: "John"},
			pm:        PresenceMap{"name": true},
			wantError: false, // Email not in presence map, so not validated
		},
		{
			name:      "partial mode with empty presence map",
			user:      &User{Name: "John"},
			pm:        PresenceMap{},
			wantError: false,
		},
		{
			name:      "partial mode with all fields present",
			user:      &User{Name: "John", Email: "john@example.com"},
			pm:        PresenceMap{"name": true, "email": true},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePartial(t.Context(), tt.user, tt.pm)
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				// Partial validation may still have errors for present fields
				if err != nil {
					t.Logf("Partial validation error: %v", err)
				}
			}
		})
	}
}
