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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper type for testing
type schemaUserImpl struct {
	Name string
}

func (s *schemaUserImpl) JSONSchema() (id string, schema string) {
	return "user", `{"type": "object"}`
}

func TestValidateAll_MultipleStrategies(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	tests := []struct {
		name      string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "user missing email - should fail multiple strategies",
			user:      User{Name: "John"}, // Missing email
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected ValidationErrors")
				assert.Greater(t, len(verr.Fields), 0, "should have validation errors")
			},
		},
		{
			name:      "user missing both fields",
			user:      User{},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.Greater(t, len(verr.Fields), 0)
			},
		},
		{
			name:      "valid user should pass",
			user:      User{Name: "John", Email: "john@example.com"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), &tt.user, WithRunAll(true))
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

func TestValidateAll_RequireAny(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	tests := []struct {
		name      string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "valid user - at least one strategy should pass",
			user:      User{Name: "John", Email: "john@example.com"},
			wantError: false,
		},
		{
			name:      "invalid user - all strategies fail",
			user:      User{},
			wantError: true,
		},
		{
			name:      "invalid user - missing email",
			user:      User{Name: "John"},
			wantError: true,
		},
		{
			name:      "invalid user - missing name",
			user:      User{Email: "john@example.com"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), &tt.user, WithRunAll(true), WithRequireAny(true))
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

func TestValidateAll_NoApplicableStrategies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		value     any
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "simple string value - no applicable strategies",
			value:     "just a string",
			wantError: false, // Should not error if no strategies apply
		},
		{
			name:      "integer value - no applicable strategies",
			value:     42,
			wantError: false,
		},
		{
			name:      "boolean value - no applicable strategies",
			value:     true,
			wantError: false,
		},
		{
			name:      "nil value - should error",
			value:     (*string)(nil),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), tt.value, WithRunAll(true))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.Nil(t, err, "should not error when no strategies apply")
			}
		})
	}
}

func TestCoerceToValidationErrors_AlreadyValidationErrors(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	tests := []struct {
		name      string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "missing both fields - should return validation errors",
			user:      User{}, // Missing fields
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var resultVerr *Error
				require.ErrorAs(t, err, &resultVerr, "expected Error")
				assert.GreaterOrEqual(t, len(resultVerr.Fields), 2, "expected at least 2 errors")
			},
		},
		{
			name:      "missing name only",
			user:      User{Email: "john@example.com"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var resultVerr *Error
				require.ErrorAs(t, err, &resultVerr)
				assert.Greater(t, len(resultVerr.Fields), 0)
			},
		},
		{
			name:      "valid user should pass",
			user:      User{Name: "John", Email: "john@example.com"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), &tt.user, WithStrategy(StrategyTags))
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

func TestCoerceToValidationErrors_WithMaxErrors(t *testing.T) {
	t.Parallel()
	type User struct {
		Field1 string `json:"field1" validate:"required"`
		Field2 string `json:"field2" validate:"required"`
		Field3 string `json:"field3" validate:"required"`
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
				require.Error(t, err)
				var resultVerr *Error
				require.ErrorAs(t, err, &resultVerr, "expected Error")
				assert.LessOrEqual(t, len(resultVerr.Fields), 2, "expected at most 2 errors")
				if len(resultVerr.Fields) == 2 {
					assert.True(t, resultVerr.Truncated, "should be truncated when maxErrors is hit")
				}
			},
		},
		{
			name:      "maxErrors = 1 should return single error",
			user:      User{},
			maxErrors: 1,
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var resultVerr *Error
				require.ErrorAs(t, err, &resultVerr)
				assert.LessOrEqual(t, len(resultVerr.Fields), 1)
			},
		},
		{
			name:      "maxErrors = 0 (unlimited) should return all errors",
			user:      User{},
			maxErrors: 0,
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var resultVerr *Error
				require.ErrorAs(t, err, &resultVerr)
				assert.GreaterOrEqual(t, len(resultVerr.Fields), 3, "expected at least 3 errors")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), &tt.user, WithMaxErrors(tt.maxErrors))
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

func TestCoerceToValidationErrors_FieldError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fe       FieldError
		wantPath string
		wantCode string
		wantMsg  string
		checkErr func(t *testing.T, target FieldError)
	}{
		{
			name:     "field error with all fields",
			fe:       FieldError{Path: "name", Code: "required", Message: "is required"},
			wantPath: "name",
			wantCode: "required",
			wantMsg:  "is required",
			checkErr: func(t *testing.T, target FieldError) {
				assert.Equal(t, "name", target.Path)
				assert.Equal(t, "required", target.Code)
				assert.Equal(t, "is required", target.Message)
			},
		},
		{
			name:     "field error with empty path",
			fe:       FieldError{Path: "", Code: "validation_error", Message: "generic error"},
			wantPath: "",
			wantCode: "validation_error",
			wantMsg:  "generic error",
			checkErr: func(t *testing.T, target FieldError) {
				assert.Empty(t, target.Path)
				assert.Equal(t, "validation_error", target.Code)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var target FieldError
			require.ErrorAs(t, tt.fe, &target, "expected FieldError")
			if tt.checkErr != nil {
				tt.checkErr(t, target)
			} else {
				assert.Equal(t, tt.wantPath, target.Path)
				assert.Equal(t, tt.wantCode, target.Code)
				assert.Equal(t, tt.wantMsg, target.Message)
			}
		})
	}
}

func TestCoerceToValidationErrors_GenericError(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name      string
		user      *User
		validator func(any) error
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "generic error should be converted to validation error",
			user: &User{},
			validator: func(v any) error {
				return errGenericValidationError
			},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected Error")
				assert.Equal(t, 1, len(verr.Fields), "expected 1 error")
				assert.Equal(t, "validation_error", verr.Fields[0].Code)
			},
		},
		{
			name: "custom error message should be preserved",
			user: &User{},
			validator: func(v any) error {
				return errCustomErrorMessage
			},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.Equal(t, 1, len(verr.Fields))
				assert.Contains(t, verr.Fields[0].Message, "custom error message")
			},
		},
		{
			name: "nil error from validator should pass",
			user: &User{Name: "John"},
			validator: func(v any) error {
				return nil
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), tt.user, WithCustomValidator(tt.validator))
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

func TestCoerceToValidationErrors_NilError(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name" validate:"required"`
	}

	tests := []struct {
		name      string
		user      *User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "valid user should return nil error",
			user:      &User{Name: "John"},
			wantError: false,
		},
		{
			name:      "invalid user should return error",
			user:      &User{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), tt.user)
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.Nil(t, err, "expected nil for valid input")
			}
		})
	}
}

func TestIsApplicable_InterfaceStrategy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		value     any
		opts      []Option
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "struct implementing Validator - valid user",
			value:     &userWithValidator{Name: "John", Email: "john@example.com"},
			opts:      []Option{WithStrategy(StrategyInterface)},
			wantError: false,
		},
		{
			name:      "struct implementing Validator - invalid user",
			value:     &userWithValidator{},
			opts:      []Option{WithStrategy(StrategyInterface)},
			wantError: true,
		},
		{
			name:  "with context and ValidatorWithContext - valid user",
			value: &userWithContextValidator{Name: "John"},
			opts: []Option{
				WithStrategy(StrategyInterface),
				WithContext(context.Background()),
			},
			wantError: false,
		},
		{
			name:  "with context and ValidatorWithContext - invalid user",
			value: &userWithContextValidator{},
			opts: []Option{
				WithStrategy(StrategyInterface),
				WithContext(context.Background()),
			},
			wantError: true,
		},
		{
			name:      "struct without validator should pass",
			value:     &struct{ Name string }{},
			opts:      []Option{WithStrategy(StrategyInterface)},
			wantError: false, // No validation to do
		},
		{
			name:      "nil pointer should error",
			value:     (*userWithValidator)(nil),
			opts:      []Option{WithStrategy(StrategyInterface)},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			err := Validate(ctx, tt.value, tt.opts...)
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

func TestIsApplicable_TagsStrategy(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name" validate:"required"`
	}

	tests := []struct {
		name      string
		value     any
		opts      []Option
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "struct with tags - missing required field",
			value:     &User{},
			opts:      []Option{WithStrategy(StrategyTags)},
			wantError: true,
		},
		{
			name:      "struct with tags - valid user",
			value:     &User{Name: "John"},
			opts:      []Option{WithStrategy(StrategyTags)},
			wantError: false,
		},
		{
			name:      "non-struct - should pass",
			value:     "string",
			opts:      []Option{WithStrategy(StrategyTags)},
			wantError: false, // No validation to do
		},
		{
			name:      "integer - should pass",
			value:     42,
			opts:      []Option{WithStrategy(StrategyTags)},
			wantError: false,
		},
		{
			name:      "nil pointer - should error",
			value:     (*User)(nil),
			opts:      nil,
			wantError: true,
		},
		{
			name:      "struct without tags - should pass",
			value:     &struct{ Name string }{Name: "John"},
			opts:      []Option{WithStrategy(StrategyTags)},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), tt.value, tt.opts...)
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

func TestIsApplicable_JSONSchemaStrategy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		value     any
		opts      []Option
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "with custom schema - should pass",
			value:     &struct{}{},
			opts:      []Option{WithStrategy(StrategyJSONSchema), WithCustomSchema("test-isapplicable", `{"type": "object"}`)},
			wantError: false,
		},
		{
			name:      "without custom schema and no provider - should pass",
			value:     &struct{}{},
			opts:      []Option{WithStrategy(StrategyJSONSchema)},
			wantError: false, // No validation to do
		},
		{
			name:      "with JSONSchemaProvider - should pass",
			value:     &schemaUserImpl{Name: "John"},
			opts:      []Option{WithStrategy(StrategyJSONSchema)},
			wantError: false,
		},
		{
			name: "with custom schema - invalid data",
			value: &struct {
				Name string `json:"name"`
			}{Name: ""},
			opts: []Option{
				WithStrategy(StrategyJSONSchema),
				WithCustomSchema("test-invalid", `{"type": "object", "properties": {"name": {"type": "string", "minLength": 1}}, "required": ["name"]}`),
			},
			wantError: true,
		},
		{
			name: "with custom schema - valid data",
			value: &struct {
				Name string `json:"name"`
			}{Name: "John"},
			opts: []Option{
				WithStrategy(StrategyJSONSchema),
				WithCustomSchema("test-valid", `{"type": "object", "properties": {"name": {"type": "string"}}}`),
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), tt.value, tt.opts...)
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

func TestDetermineStrategy_Auto(t *testing.T) {
	t.Parallel()
	type TagUser struct {
		Name string `json:"name" validate:"required"`
	}
	type SimpleStruct struct {
		Name string
	}

	tests := []struct {
		name      string
		value     any
		opts      []Option
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "struct with Validator interface - should prefer interface",
			value:     &userWithValidator{Name: "John", Email: "john@example.com"},
			opts:      nil, // Auto strategy
			wantError: false,
		},
		{
			name:      "struct with tags but no interface - should use tags",
			value:     &TagUser{},
			opts:      nil,  // Auto strategy
			wantError: true, // Missing required field
		},
		{
			name:      "struct with tags - valid user",
			value:     &TagUser{Name: "John"},
			opts:      nil,
			wantError: false,
		},
		{
			name:      "struct with JSON Schema - custom schema",
			value:     &schemaUserImpl{Name: "John"},
			opts:      []Option{WithCustomSchema("test-auto-schema", `{"type": "object"}`)},
			wantError: false,
		},
		{
			name:      "struct with JSONSchemaProvider - should use schema",
			value:     &schemaUserImpl{Name: "John"},
			opts:      nil, // Auto strategy
			wantError: false,
		},
		{
			name:      "simple struct - should pass",
			value:     &SimpleStruct{},
			opts:      nil,   // Auto strategy
			wantError: false, // No validation rules
		},
		{
			name:      "non-struct value - should pass",
			value:     "string",
			opts:      nil,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), tt.value, tt.opts...)
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

func TestValidateByStrategy(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name" validate:"required"`
	}

	tests := []struct {
		name      string
		user      *User
		strategy  Strategy
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "tags strategy - missing name",
			user:      &User{}, // Missing name
			strategy:  StrategyTags,
			wantError: true,
		},
		{
			name:      "tags strategy - valid user",
			user:      &User{Name: "John"},
			strategy:  StrategyTags,
			wantError: false,
		},
		{
			name:      "interface strategy - no validator",
			user:      &User{},
			strategy:  StrategyInterface,
			wantError: false, // No validator, so no validation to do
		},
		{
			name:      "JSON Schema strategy - no schema",
			user:      &User{},
			strategy:  StrategyJSONSchema,
			wantError: false, // No schema, so no validation to do
		},
		{
			name:      "auto strategy - should detect tags",
			user:      &User{},
			strategy:  StrategyAuto,
			wantError: true, // Auto should detect tags and fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), tt.user, WithStrategy(tt.strategy))
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

func TestValidateWithInterface_AutoStrategy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		user      *userWithValidator
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "valid user - auto should detect interface",
			user:      &userWithValidator{Name: "John", Email: "john@example.com"},
			wantError: false, // No strategy specified - should auto-detect
		},
		{
			name:      "invalid user - missing name",
			user:      &userWithValidator{Email: "john@example.com"},
			wantError: true, // Auto should use interface validation
		},
		{
			name:      "invalid user - missing email",
			user:      &userWithValidator{Name: "John"},
			wantError: true,
		},
		{
			name:      "invalid user - missing both",
			user:      &userWithValidator{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), tt.user) // Auto strategy
			if tt.wantError {
				require.Error(t, err, "expected validation error")
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAll_AllStrategiesCombined(t *testing.T) {
	t.Parallel()
	type CombinedUser struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	tests := []struct {
		name      string
		user      *CombinedUser
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "valid user should pass",
			user:      &CombinedUser{Name: "John", Email: "john@example.com"},
			wantError: false,
		},
		{
			name:      "invalid user - missing email",
			user:      &CombinedUser{Name: "John"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected Error")
				assert.Greater(t, len(verr.Fields), 0, "should have validation errors")
			},
		},
		{
			name:      "invalid user - missing name",
			user:      &CombinedUser{Email: "john@example.com"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.Greater(t, len(verr.Fields), 0)
			},
		},
		{
			name:      "invalid user - missing both",
			user:      &CombinedUser{},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.Greater(t, len(verr.Fields), 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), tt.user, WithRunAll(true))
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

func TestDetermineStrategy_PriorityMatrix(t *testing.T) {
	t.Parallel()
	type TagUser struct {
		Name string `json:"name" validate:"required"`
	}
	type SimpleUser struct {
		Name string `json:"name"`
	}
	type SimpleStruct struct {
		Name string
	}

	tests := []struct {
		name      string
		value     any
		opts      []Option
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "interface only - should prefer interface",
			value:     &userWithValidator{Name: "John", Email: "john@example.com"},
			opts:      nil, // Auto strategy
			wantError: false,
		},
		{
			name:      "tags only - should use tags",
			value:     &TagUser{},
			opts:      nil,  // Auto strategy
			wantError: true, // Missing required field
		},
		{
			name:      "tags only - valid user",
			value:     &TagUser{Name: "John"},
			opts:      nil,
			wantError: false,
		},
		{
			name:      "JSON Schema only - with custom schema",
			value:     &SimpleUser{},
			opts:      []Option{WithCustomSchema("test-priority-matrix", `{"type": "object"}`)},
			wantError: false,
		},
		{
			name:      "interface + tags - should prefer interface",
			value:     &userWithValidator{Name: "John", Email: "john@example.com"},
			opts:      nil, // Auto strategy
			wantError: false,
		},
		{
			name:      "JSON Schema only - with JSONSchemaProvider",
			value:     &schemaUserImpl{Name: "John"},
			opts:      nil, // Auto strategy
			wantError: false,
		},
		{
			name:      "default for simple struct - should pass",
			value:     &SimpleStruct{},
			opts:      nil, // Auto strategy
			wantError: false,
		},
		{
			name:      "non-struct value - should pass",
			value:     "string",
			opts:      nil,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(context.Background(), tt.value, tt.opts...)
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
