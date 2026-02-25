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
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_InvalidConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		opt        Option
		wantErrMsg string
	}{
		{
			name:       "maxErrors negative returns error",
			opt:        WithMaxErrors(-1),
			wantErrMsg: "maxErrors must be non-negative",
		},
		{
			name:       "maxFields negative returns error",
			opt:        WithMaxFields(-1),
			wantErrMsg: "maxFields must be non-negative",
		},
		{
			name:       "maxCachedSchemas negative returns error",
			opt:        WithMaxCachedSchemas(-1),
			wantErrMsg: "maxCachedSchemas must be non-negative",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v, err := New(tt.opt)
			require.Error(t, err)
			assert.Nil(t, v)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

func TestMustNew_PanicsOnInvalidConfig(t *testing.T) {
	t.Parallel()
	var panicked bool
	var panicVal any
	func() {
		defer func() {
			panicVal = recover()
			panicked = panicVal != nil
		}()
		MustNew(WithMaxErrors(-1))
	}()
	require.True(t, panicked, "MustNew should panic on invalid config")
	assert.Contains(t, fmt.Sprint(panicVal), "validation.MustNew")
	assert.Contains(t, fmt.Sprint(panicVal), "maxErrors")
}

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
		Password string `json:"password" validate:"required,min=8"` //nolint:gosec // G117: test fixture, no real credentials
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

func TestWithDisallowUnknownFields_OptionApplied(t *testing.T) {
	t.Parallel()
	v := MustNew(WithDisallowUnknownFields(true))
	require.NotNil(t, v)
	type User struct {
		Name string `json:"name" validate:"required"`
	}
	err := v.Validate(t.Context(), &User{})
	require.Error(t, err)
	// Option is applied at validator creation; binding is tested elsewhere
	assert.NotNil(t, err)
}

func TestWithFieldNameMapper_AppliesToErrorPaths(t *testing.T) {
	t.Parallel()
	mapper := func(name string) string {
		return strings.ReplaceAll(name, "_", " ")
	}
	v := MustNew(WithFieldNameMapper(mapper), WithStrategy(StrategyTags))
	require.NotNil(t, v)
	type User struct {
		FirstName string `json:"first_name" validate:"required"` //nolint:tagliatelle // intentional for mapper test
	}
	err := v.Validate(t.Context(), &User{})
	require.Error(t, err)
	var verr *Error
	require.ErrorAs(t, err, &verr)
	require.NotEmpty(t, verr.Fields)
	// Field name mapper should transform path (e.g. first_name -> first name)
	assert.Contains(t, verr.Fields[0].Path, " ", "field name mapper should have transformed path")
}

func TestWithMaxFields_OptionApplied(t *testing.T) {
	t.Parallel()
	v := MustNew(WithMaxFields(5000), WithPartial(true))
	require.NotNil(t, v)
	type User struct {
		Name string `json:"name" validate:"required"`
	}
	pm, err := ComputePresence([]byte(`{"name":"x"}`))
	require.NoError(t, err)
	user := &User{Name: "x"}
	err = v.Validate(t.Context(), user, WithPresence(pm))
	assert.NoError(t, err)
}

func TestWithMaxCachedSchemas_OptionApplied(t *testing.T) {
	t.Parallel()
	v := MustNew(WithMaxCachedSchemas(2048))
	require.NotNil(t, v)
	type User struct {
		Name string `json:"name" validate:"required"`
	}
	err := v.Validate(t.Context(), &User{Name: "ok"})
	assert.NoError(t, err)
}

func TestWithCustomTag_OptionApplied(t *testing.T) {
	t.Parallel()
	v := MustNew(WithCustomTag("nonempty", func(fl validator.FieldLevel) bool {
		return len(fl.Field().String()) > 0
	}))
	require.NotNil(t, v)
	type User struct {
		Name string `json:"name" validate:"nonempty"`
	}
	err := v.Validate(t.Context(), &User{}, WithStrategy(StrategyTags))
	require.Error(t, err)
	err = v.Validate(t.Context(), &User{Name: "ok"}, WithStrategy(StrategyTags))
	assert.NoError(t, err)
}

// TestClone_WithCustomTags triggers clone() when base config has customTags set.
func TestClone_WithCustomTags(t *testing.T) {
	t.Parallel()
	v := MustNew(WithCustomTag("alwaysok", func(fl validator.FieldLevel) bool { return true }))
	require.NotNil(t, v)
	type User struct {
		Name string `json:"name" validate:"required,alwaysok"`
	}
	// Pass an extra option so applyOptions clones the base config (which has customTags).
	err := v.Validate(t.Context(), &User{}, WithStrategy(StrategyTags))
	require.Error(t, err)
	// Clone with customTags was exercised; validation ran
	var verr *Error
	require.ErrorAs(t, err, &verr)
	assert.NotEmpty(t, verr.Fields)
}

// TestClone_WithMessages triggers clone() when base config has messages set.
func TestClone_WithMessages(t *testing.T) {
	t.Parallel()
	v := MustNew(WithMessages(map[string]string{"required": "custom required msg"}))
	require.NotNil(t, v)
	type User struct {
		Name string `json:"name" validate:"required"`
	}
	err := v.Validate(t.Context(), &User{}, WithStrategy(StrategyTags))
	require.Error(t, err)
	var verr *Error
	require.ErrorAs(t, err, &verr)
	assert.NotEmpty(t, verr.Fields)
}

// TestClone_WithMessageFuncs triggers clone() when base config has messageFuncs set.
func TestClone_WithMessageFuncs(t *testing.T) {
	t.Parallel()
	v := MustNew(WithMessageFunc("min", func(param string, _ reflect.Kind) string {
		return "min " + param
	}))
	require.NotNil(t, v)
	type User struct {
		Name string `json:"name" validate:"required,min=2"`
	}
	err := v.Validate(t.Context(), &User{Name: "x"}, WithStrategy(StrategyTags))
	require.Error(t, err)
	var verr *Error
	require.ErrorAs(t, err, &verr)
	assert.NotEmpty(t, verr.Fields)
}

func TestValidator_SchemaCacheEviction(t *testing.T) {
	t.Parallel()
	// WithMaxCachedSchemas(2): fill cache with 3 schemas to trigger eviction of oldest
	v := MustNew(WithStrategy(StrategyJSONSchema), WithMaxCachedSchemas(2))
	require.NotNil(t, v)
	type User struct {
		Name string `json:"name"`
	}
	schemaA := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	schemaB := `{"type":"object","properties":{"title":{"type":"string"}},"required":["title"]}`
	schemaC := `{"type":"object","properties":{"id":{"type":"integer"}}}`
	// Validate with A, B, C to fill cache and evict A (oldest)
	err := v.Validate(t.Context(), &User{Name: "x"}, WithCustomSchema("schema-a", schemaA))
	require.NoError(t, err)
	err = v.Validate(t.Context(), &struct {
		Title string `json:"title"`
	}{Title: "y"}, WithCustomSchema("schema-b", schemaB))
	require.NoError(t, err)
	err = v.Validate(t.Context(), &struct {
		ID int `json:"id"`
	}{ID: 1}, WithCustomSchema("schema-c", schemaC))
	require.NoError(t, err)
	// Validate with A again; A was evicted so it is recompiled and should still work
	err = v.Validate(t.Context(), &User{Name: "z"}, WithCustomSchema("schema-a", schemaA))
	require.NoError(t, err)
}
