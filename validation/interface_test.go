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

package validation

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testContextKey string

const testLocaleKey testContextKey = "locale"

// Static errors for validation tests
var (
	errNameRequired  = errors.New("name is required")
	errEmailRequired = errors.New("email is required")
	errShouldNotUse  = errors.New("should not use Validate()")
	errNameTooShort  = errors.New("نام باید حداقل ۳ کاراکتر باشد")
)

// Test structs implementing Validator interface
type userWithValidator struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (u *userWithValidator) Validate() error {
	if u.Name == "" {
		return errNameRequired
	}
	if u.Email == "" {
		return errEmailRequired
	}

	return nil
}

type userWithValueValidator struct {
	Name string `json:"name"`
}

func (u userWithValueValidator) Validate() error {
	if u.Name == "" {
		return errNameRequired
	}

	return nil
}

// Test structs implementing ValidatorWithContext interface
type userWithContextValidator struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (u *userWithContextValidator) ValidateContext(ctx context.Context) error {
	if u.Name == "" {
		return errNameRequired
	}
	// Check context for locale
	if locale := ctx.Value(testLocaleKey); locale != nil {
		localeStr, ok := locale.(string)
		if ok && localeStr == "fa" && len(u.Name) < 3 {
			return errNameTooShort
		}
	}

	return nil
}

type userWithValueContextValidator struct {
	Name string `json:"name"`
}

func (u userWithValueContextValidator) ValidateContext(_ context.Context) error {
	if u.Name == "" {
		return errNameRequired
	}

	return nil
}

// Test struct with both interfaces (should prefer ValidatorWithContext)
type userWithBoth struct {
	Name string `json:"name"`
}

func (u *userWithBoth) Validate() error {
	return errShouldNotUse
}

func (u *userWithBoth) ValidateContext(_ context.Context) error {
	if u.Name == "" {
		return errNameRequired
	}

	return nil
}

func TestValidateWithInterface_PointerReceiver(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		user      *userWithValidator
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "valid user with both fields",
			user:      &userWithValidator{Name: "John", Email: "john@example.com"},
			wantError: false,
		},
		{
			name:      "invalid user - missing name",
			user:      &userWithValidator{Email: "john@example.com"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.True(t, verr.HasCode("validation_error"))
			},
		},
		{
			name:      "invalid user - missing email",
			user:      &userWithValidator{Name: "John"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.True(t, verr.HasCode("validation_error"))
			},
		},
		{
			name:      "invalid user - missing both fields",
			user:      &userWithValidator{},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.True(t, verr.HasCode("validation_error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.user, WithStrategy(StrategyInterface))
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

func TestValidateWithInterface_ValueReceiver(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		user      userWithValueValidator
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "valid user with name",
			user:      userWithValueValidator{Name: "John"},
			wantError: false,
		},
		{
			name:      "invalid user - missing name",
			user:      userWithValueValidator{},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.True(t, verr.HasCode("validation_error"))
			},
		},
		{
			name:      "valid user with empty string name",
			user:      userWithValueValidator{Name: ""},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.user, WithStrategy(StrategyInterface))
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

func TestValidateWithInterface_WithContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		user      *userWithContextValidator
		setupCtx  func(*testing.T) context.Context
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "valid user with English locale",
			user:      &userWithContextValidator{Name: "John"},
			setupCtx:  func(t *testing.T) context.Context { return context.WithValue(t.Context(), testLocaleKey, "en") },
			wantError: false,
		},
		{
			name:      "invalid user - missing name",
			user:      &userWithContextValidator{},
			setupCtx:  func(t *testing.T) context.Context { return context.WithValue(t.Context(), testLocaleKey, "en") },
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
		{
			name:      "invalid user - short name for Farsi locale",
			user:      &userWithContextValidator{Name: "Jo"},
			setupCtx:  func(t *testing.T) context.Context { return context.WithValue(t.Context(), testLocaleKey, "fa") },
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
		{
			name:      "valid long name for Farsi locale",
			user:      &userWithContextValidator{Name: "محمد"},
			setupCtx:  func(t *testing.T) context.Context { return context.WithValue(t.Context(), testLocaleKey, "fa") },
			wantError: false,
		},
		{
			name:      "valid name with no locale in context",
			user:      &userWithContextValidator{Name: "John"},
			setupCtx:  func(t *testing.T) context.Context { return t.Context() },
			wantError: false,
		},
		{
			name:      "valid name with different locale",
			user:      &userWithContextValidator{Name: "Jo"},
			setupCtx:  func(t *testing.T) context.Context { return context.WithValue(t.Context(), testLocaleKey, "en") },
			wantError: false, // English locale doesn't have length restriction
		},
		{
			name:      "invalid - empty name with context",
			user:      &userWithContextValidator{Name: ""},
			setupCtx:  func(t *testing.T) context.Context { return context.WithValue(t.Context(), testLocaleKey, "en") },
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := tt.setupCtx(t)
			err := Validate(ctx, tt.user, WithStrategy(StrategyInterface), WithContext(ctx))
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

func TestValidateWithInterface_ValueReceiverWithContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		user      userWithValueContextValidator
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "valid user with name",
			user:      userWithValueContextValidator{Name: "John"},
			wantError: false,
		},
		{
			name:      "invalid user - missing name",
			user:      userWithValueContextValidator{},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
		{
			name:      "invalid user - empty name",
			user:      userWithValueContextValidator{Name: ""},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			err := Validate(ctx, tt.user, WithStrategy(StrategyInterface), WithContext(ctx))
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

func TestValidateWithInterface_PrefersContextOverValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		user      *userWithBoth
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "valid user - should use ValidateContext",
			user:      &userWithBoth{Name: "John"},
			wantError: false,
			checkErr: func(t *testing.T, err error) {
				// If Validate() was called, it would return "should not use Validate()"
				// Since wantError is false, err should be nil
				require.NoError(t, err, "valid user should not have errors")
				// Defensive check: if error exists, ensure it's not from Validate()
				if err != nil {
					assert.NotContains(t, err.Error(), "should not use Validate()",
						"ValidateContext should be preferred over Validate")
				}
			},
		},
		{
			name:      "invalid user - missing name",
			user:      &userWithBoth{Name: ""},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				// Should not have used Validate() which returns "should not use Validate()"
				assert.NotContains(t, err.Error(), "should not use Validate()",
					"ValidateContext should be preferred over Validate")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			err := Validate(ctx, tt.user, WithStrategy(StrategyInterface), WithContext(ctx))
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tt.checkErr != nil {
				tt.checkErr(t, err)
			}
		})
	}
}

func TestValidateWithInterface_NoValidator(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		value     any
		wantError bool
	}{
		{
			name:      "struct without validator should pass",
			value:     &struct{ Name string }{Name: "test"},
			wantError: false,
		},
		{
			name:      "value struct without validator should pass",
			value:     struct{ Name string }{Name: "test"},
			wantError: false,
		},
		{
			name:      "primitive type without validator should pass",
			value:     "test",
			wantError: false,
		},
		{
			name:      "slice without validator should pass",
			value:     []string{"test"},
			wantError: false,
		},
		{
			name:      "map without validator should pass",
			value:     map[string]string{"key": "value"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.value, WithStrategy(StrategyInterface))
			if tt.wantError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWithInterface_ErrorCoercion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		user     *userWithValidator
		checkErr func(t *testing.T, err error)
	}{
		{
			name: "missing name should be coerced to validation.Error",
			user: &userWithValidator{Email: "test@example.com"},
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr, "should be able to unwrap to validation.Error")
			},
		},
		{
			name: "missing email should be coerced to validation.Error",
			user: &userWithValidator{Name: "John"},
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr, "should be able to unwrap to validation.Error")
			},
		},
		{
			name: "missing both fields should be coerced to validation.Error",
			user: &userWithValidator{},
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr, "should be able to unwrap to validation.Error")
				// Should contain error about name (first check in Validate method)
				assert.True(t, verr.HasCode("validation_error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.user, WithStrategy(StrategyInterface))
			require.Error(t, err, "expected validation error")
			if tt.checkErr != nil {
				tt.checkErr(t, err)
			}
		})
	}
}

func TestValidateWithInterface_NilPointer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		value     any
		wantError bool
	}{
		{
			name:      "nil pointer to struct with validator",
			value:     (*userWithValidator)(nil),
			wantError: true,
		},
		{
			name:      "nil pointer to struct with context validator",
			value:     (*userWithContextValidator)(nil),
			wantError: true,
		},
		{
			name:      "nil pointer to struct with both validators",
			value:     (*userWithBoth)(nil),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.value, WithStrategy(StrategyInterface))
			if tt.wantError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWithInterface_ContextWithoutWithContextOption(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		user      *userWithContextValidator
		setupCtx  func(*testing.T) context.Context
		wantError bool
	}{
		{
			name:      "context provided but WithContext not called - should still use ValidateContext",
			user:      &userWithContextValidator{Name: "John"},
			setupCtx:  func(t *testing.T) context.Context { return context.WithValue(t.Context(), testLocaleKey, "en") },
			wantError: false,
		},
		{
			name:      "context provided but WithContext not called - invalid user",
			user:      &userWithContextValidator{},
			setupCtx:  func(t *testing.T) context.Context { return context.WithValue(t.Context(), testLocaleKey, "en") },
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Note: When context is provided to Validate(), it should still use ValidateContext
			// if WithContext is not explicitly called, the context from Validate() is used
			ctx := tt.setupCtx(t)
			err := Validate(ctx, tt.user, WithStrategy(StrategyInterface))
			if tt.wantError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWithInterface_AutoStrategyDetection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		user      *userWithValidator
		wantError bool
	}{
		{
			name:      "auto strategy should detect interface validator",
			user:      &userWithValidator{Name: "John", Email: "john@example.com"},
			wantError: false,
		},
		{
			name:      "auto strategy should detect interface validator and return error",
			user:      &userWithValidator{Name: "John"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Using StrategyAuto should still work with interface validators
			err := Validate(t.Context(), tt.user, WithStrategy(StrategyAuto))
			if tt.wantError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWithInterface_ErrorUnwrapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		user     *userWithValidator
		checkErr func(t *testing.T, err error)
	}{
		{
			name: "error should unwrap to ErrValidation",
			user: &userWithValidator{},
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr)
				unwrapped := verr.Unwrap()
				assert.ErrorIs(t, unwrapped, ErrValidation)
			},
		},
		{
			name: "error should be unwrappable with errors.Is",
			user: &userWithValidator{Email: "test@example.com"},
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrValidation)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.user, WithStrategy(StrategyInterface))
			require.Error(t, err)
			if tt.checkErr != nil {
				tt.checkErr(t, err)
			}
		})
	}
}

func TestValidateWithInterface_ContextCancellation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		setupCtx  func(*testing.T) context.Context
		user      *userWithContextValidator
		wantError bool
	}{
		{
			name: "cancelled context should still validate",
			setupCtx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel() // Cancel immediately

				return ctx
			},
			user:      &userWithContextValidator{Name: "John"},
			wantError: false, // Validation should still work even with cancelled context
		},
		{
			name: "cancelled context with invalid user",
			setupCtx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				return ctx
			},
			user:      &userWithContextValidator{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := tt.setupCtx(t)
			err := Validate(ctx, tt.user, WithStrategy(StrategyInterface), WithContext(ctx))
			if tt.wantError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWithInterface_NoContextProvided(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		user      *userWithContextValidator
		wantError bool
	}{
		{
			name:      "no context - should fall back to Validate if available",
			user:      &userWithContextValidator{Name: "John"},
			wantError: false, // userWithContextValidator doesn't implement Validate, so should pass
		},
		{
			name:      "no context - invalid user",
			user:      &userWithContextValidator{},
			wantError: true, // Should still validate using ValidateContext if context is nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Not providing context - should try ValidateContext first, then Validate
			err := Validate(t.Context(), tt.user, WithStrategy(StrategyInterface))
			if tt.wantError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
