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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_NilSlice(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string   `json:"name" validate:"required"`
		Tags  []string `json:"tags" validate:"required"`
		Items []string `json:"items"`
	}

	tests := []struct {
		name      string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "nil slice should fail required validation",
			user:      User{Name: "John", Tags: nil},
			wantError: true,
		},
		{
			name:      "empty slice with required tag should pass",
			user:      User{Name: "John", Tags: []string{}},
			wantError: false, // go-playground/validator's "required" only checks for nil, not empty
		},
		{
			name:      "valid slice should pass",
			user:      User{Name: "John", Tags: []string{"tag1"}, Items: nil},
			wantError: false,
		},
		{
			name:      "nil items without required tag should pass",
			user:      User{Name: "John", Tags: []string{"tag1"}, Items: nil},
			wantError: false,
		},
		{
			name:      "empty items without required tag should pass",
			user:      User{Name: "John", Tags: []string{"tag1"}, Items: []string{}},
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

func TestValidate_NilMap(t *testing.T) {
	t.Parallel()
	type User struct {
		Name     string            `json:"name" validate:"required"`
		Metadata map[string]string `json:"metadata" validate:"required"`
	}

	tests := []struct {
		name      string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "nil map should fail required validation",
			user:      User{Name: "John", Metadata: nil},
			wantError: true,
		},
		{
			name:      "empty map with required tag should pass",
			user:      User{Name: "John", Metadata: map[string]string{}},
			wantError: false, // go-playground/validator's "required" only checks for nil, not empty
		},
		{
			name:      "valid map should pass",
			user:      User{Name: "John", Metadata: map[string]string{"key": "value"}},
			wantError: false,
		},
		{
			name:      "map with multiple entries should pass",
			user:      User{Name: "John", Metadata: map[string]string{"key1": "value1", "key2": "value2"}},
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

func TestValidate_DeeplyNestedStructures(t *testing.T) {
	t.Parallel()
	type Level5 struct {
		Value string `json:"value" validate:"required"`
	}
	type Level4 struct {
		Level5 Level5 `json:"level5"`
	}
	type Level3 struct {
		Level4 Level4 `json:"level4"`
	}
	type Level2 struct {
		Level3 Level3 `json:"level3"`
	}
	type Level1 struct {
		Level2 Level2 `json:"level2"`
	}

	tests := []struct {
		name      string
		value     Level1
		wantError bool
		wantPath  string
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "valid deeply nested structure",
			value: Level1{
				Level2: Level2{
					Level3: Level3{
						Level4: Level4{
							Level5: Level5{Value: "test"},
						},
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid - missing value at level 5",
			value: Level1{
				Level2: Level2{
					Level3: Level3{
						Level4: Level4{
							Level5: Level5{},
						},
					},
				},
			},
			wantError: true,
			wantPath:  "level2.level3.level4.level5.value",
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected ValidationErrors")
				found := false
				for _, e := range verr.Fields {
					if e.Path == "level2.level3.level4.level5.value" {
						found = true
						break
					}
				}
				assert.True(t, found, "expected error path for deeply nested field")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.value, WithStrategy(StrategyTags))
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

func TestValidate_Concurrent(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	const numGoroutines = 100
	const numValidationsPerGoroutine = 10

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*numValidationsPerGoroutine)

	// Run concurrent validations
	for i := range numGoroutines {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			for range numValidationsPerGoroutine {
				user := User{
					Name:  "John",
					Email: "john@example.com",
				}
				err := Validate(t.Context(), &user, WithStrategy(StrategyTags))
				if err != nil {
					errChan <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for any errors
	errCount := 0
	for err := range errChan {
		t.Errorf("unexpected validation error: %v", err)
		errCount++
	}

	assert.Equal(t, 0, errCount, "should have no errors during concurrent validation")
}

func TestValidate_ConcurrentWithCache(t *testing.T) {
	t.Parallel()
	// Test concurrent schema cache access
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		}
	}`

	type User struct {
		Name string `json:"name"`
	}

	const numGoroutines = 50
	const numValidationsPerGoroutine = 20

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*numValidationsPerGoroutine)

	ctx := t.Context()
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Use numeric ID that's URL-safe
			schemaID := fmt.Sprintf("test-schema-%d", id)
			for range numValidationsPerGoroutine {
				user := User{Name: "John"}
				err := Validate(ctx, &user, WithStrategy(StrategyJSONSchema), WithCustomSchema(schemaID, schema))
				if err != nil {
					errChan <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	errCount := 0
	for err := range errChan {
		t.Errorf("unexpected validation error: %v", err)
		errCount++
	}

	assert.Equal(t, 0, errCount, "should have no errors during concurrent schema validation")
}

func TestValidateWithContext_Cancellation(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name      string
		setupCtx  func(*testing.T) context.Context
		user      *User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "cancelled context should still allow validation",
			setupCtx: func(t *testing.T) context.Context {
				t.Helper()
				ctx, cancel := context.WithCancel(t.Context())
				cancel() // Cancel immediately

				return ctx
			},
			user:      &User{Name: "John"},
			wantError: false, // Validation should still work even with cancelled context
		},
		{
			name: "cancelled context with invalid user",
			setupCtx: func(t *testing.T) context.Context {
				t.Helper()
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				return ctx
			},
			user:      &User{},
			wantError: false, // Most validators don't check context cancellation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := tt.setupCtx(t)
			err := Validate(ctx, tt.user, WithContext(ctx), WithStrategy(StrategyTags))
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

func TestValidateWithContext_Timeout(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name" validate:"required"`
	}

	tests := []struct {
		name      string
		setupCtx  func(*testing.T) (context.Context, context.CancelFunc)
		user      *User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "validation should complete before timeout",
			setupCtx: func(t *testing.T) (context.Context, context.CancelFunc) {
				t.Helper()
				return context.WithTimeout(t.Context(), 100*time.Millisecond)
			},
			user:      &User{Name: "John"},
			wantError: false,
		},
		{
			name: "validation should work after timeout",
			setupCtx: func(t *testing.T) (context.Context, context.CancelFunc) {
				t.Helper()
				ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
				time.Sleep(20 * time.Millisecond) // Wait for timeout

				return ctx, cancel
			},
			user:      &User{Name: "John"},
			wantError: false, // Should still work (validators typically don't check context)
		},
		{
			name: "invalid user with timeout context",
			setupCtx: func(t *testing.T) (context.Context, context.CancelFunc) {
				t.Helper()
				return context.WithTimeout(t.Context(), 100*time.Millisecond)
			},
			user:      &User{},
			wantError: true, // Missing required field
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := tt.setupCtx(t)
			defer cancel()
			err := Validate(ctx, tt.user, WithContext(ctx), WithStrategy(StrategyTags))
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

func TestValidationErrors_ErrorsAs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		setup     func() Error
		wantCount int
		checkErr  func(t *testing.T, verr Error)
	}{
		{
			name: "errors.As should work with validation.Error",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)
				verr.Add("email", "email", "invalid email", nil)

				return verr
			},
			wantCount: 2,
			checkErr: func(t *testing.T, verr Error) {
				t.Helper()
				var target *Error
				require.ErrorAs(t, &verr, &target, "errors.As should work with validation.Error")
				assert.Len(t, target.Fields, 2)
			},
		},
		{
			name: "errors.Is should work with validation.Error",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)

				return verr
			},
			wantCount: 1,
			checkErr: func(t *testing.T, verr Error) {
				t.Helper()
				assert.ErrorIs(t, &verr, ErrValidation, "errors.Is should work with validation.Error")
			},
		},
		{
			name: "empty error should still work with errors.Is",
			setup: func() Error {
				return Error{}
			},
			wantCount: 0,
			checkErr: func(t *testing.T, verr Error) {
				t.Helper()
				assert.ErrorIs(t, &verr, ErrValidation)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			verr := tt.setup()
			if tt.checkErr != nil {
				tt.checkErr(t, verr)
			} else {
				var target *Error
				require.ErrorAs(t, &verr, &target)
				assert.Len(t, target.Fields, tt.wantCount)
			}
		})
	}
}

func TestFieldError_ErrorsIs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fe       FieldError
		wantPath string
		wantCode string
		checkErr func(t *testing.T, fe FieldError)
	}{
		{
			name: "errors.Is should work with FieldError",
			fe: FieldError{
				Path:    "name",
				Code:    "required",
				Message: "is required",
			},
			wantPath: "name",
			wantCode: "required",
			checkErr: func(t *testing.T, fe FieldError) {
				t.Helper()
				assert.ErrorIs(t, fe, ErrValidation, "errors.Is should work with FieldError")
			},
		},
		{
			name: "errors.As should work with FieldError",
			fe: FieldError{
				Path:    "email",
				Code:    "email",
				Message: "invalid email",
			},
			wantPath: "email",
			wantCode: "email",
			checkErr: func(t *testing.T, fe FieldError) {
				t.Helper()
				var target FieldError
				require.ErrorAs(t, fe, &target, "errors.As should work with FieldError")
				assert.Equal(t, "email", target.Path)
				assert.Equal(t, "email", target.Code)
			},
		},
		{
			name: "field error with empty path should work",
			fe: FieldError{
				Path:    "",
				Code:    "validation_error",
				Message: "generic error",
			},
			wantPath: "",
			wantCode: "validation_error",
			checkErr: func(t *testing.T, fe FieldError) {
				t.Helper()
				require.ErrorIs(t, fe, ErrValidation)
				var target FieldError
				require.ErrorAs(t, fe, &target)
				assert.Empty(t, target.Path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.checkErr != nil {
				tt.checkErr(t, tt.fe)
			} else {
				require.ErrorIs(t, tt.fe, ErrValidation)
				var target FieldError
				require.ErrorAs(t, tt.fe, &target)
				assert.Equal(t, tt.wantPath, target.Path)
				assert.Equal(t, tt.wantCode, target.Code)
			}
		})
	}
}

func TestValidationErrors_UnwrapChain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func() Error
		checkErr func(t *testing.T, verr Error)
	}{
		{
			name: "Unwrap should return ErrValidation",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)

				return verr
			},
			checkErr: func(t *testing.T, verr Error) {
				t.Helper()
				err := verr.Unwrap()
				assert.ErrorIs(t, err, ErrValidation, "Unwrap should return ErrValidation")
			},
		},
		{
			name: "errors.Is should work through unwrap chain",
			setup: func() Error {
				var verr Error
				verr.Add("name", "required", "is required", nil)

				return verr
			},
			checkErr: func(t *testing.T, verr Error) {
				t.Helper()
				assert.ErrorIs(t, &verr, ErrValidation, "errors.Is should work through unwrap chain")
			},
		},
		{
			name: "nested wrapping should work",
			setup: func() Error {
				var verr Error
				verr.Add("email", "email", "invalid email", nil)

				return verr
			},
			checkErr: func(t *testing.T, verr Error) {
				t.Helper()
				outerErr := errOuterError
				wrapped := fmt.Errorf("%w: %w", outerErr, &verr)
				// Note: FieldError and validation.Error already implement Unwrap
				// This test verifies the chain works
				require.ErrorIs(t, &verr, ErrValidation)
				require.ErrorIs(t, wrapped, ErrValidation, "wrapped error should still be ErrValidation")
				_ = wrapped // Suppress unused variable warning
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			verr := tt.setup()
			if tt.checkErr != nil {
				tt.checkErr(t, verr)
			}
		})
	}
}

func TestValidate_ManyErrors(t *testing.T) {
	t.Parallel()
	// Create a struct with many fields
	type User struct {
		Field1  string `json:"field1" validate:"required"`
		Field2  string `json:"field2" validate:"required"`
		Field3  string `json:"field3" validate:"required"`
		Field4  string `json:"field4" validate:"required"`
		Field5  string `json:"field5" validate:"required"`
		Field6  string `json:"field6" validate:"required"`
		Field7  string `json:"field7" validate:"required"`
		Field8  string `json:"field8" validate:"required"`
		Field9  string `json:"field9" validate:"required"`
		Field10 string `json:"field10" validate:"required"`
		Field11 string `json:"field11" validate:"required"`
		Field12 string `json:"field12" validate:"required"`
		Field13 string `json:"field13" validate:"required"`
		Field14 string `json:"field14" validate:"required"`
		Field15 string `json:"field15" validate:"required"`
		Field16 string `json:"field16" validate:"required"`
		Field17 string `json:"field17" validate:"required"`
		Field18 string `json:"field18" validate:"required"`
		Field19 string `json:"field19" validate:"required"`
		Field20 string `json:"field20" validate:"required"`
	}

	tests := []struct {
		name          string
		user          User
		maxErrors     int
		wantError     bool
		wantMaxLen    int
		wantMinLen    int
		wantTruncated bool
		checkErr      func(t *testing.T, err error)
	}{
		{
			name:          "maxErrors = 5 should limit errors",
			user:          User{}, // All fields missing
			maxErrors:     5,
			wantError:     true,
			wantMaxLen:    5,
			wantTruncated: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected ValidationErrors")
				assert.LessOrEqual(t, len(verr.Fields), 5, "expected at most 5 errors")
				assert.True(t, verr.Truncated, "should be truncated")
			},
		},
		{
			name:          "maxErrors = 0 (unlimited) should return all errors",
			user:          User{},
			maxErrors:     0,
			wantError:     true,
			wantMinLen:    15,
			wantTruncated: false,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				assert.GreaterOrEqual(t, len(verr.Fields), 15, "expected at least 15 errors with unlimited")
			},
		},
		{
			name:          "maxErrors = 1 should return single error",
			user:          User{},
			maxErrors:     1,
			wantError:     true,
			wantMaxLen:    1,
			wantTruncated: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.LessOrEqual(t, len(verr.Fields), 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.user, WithStrategy(StrategyTags), WithMaxErrors(tt.maxErrors))
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

func TestValidate_DeepRecursion(t *testing.T) {
	t.Parallel()
	// Test that deeply nested structures don't cause stack overflow
	type Nested struct {
		Value string  `json:"value" validate:"required"`
		Next  *Nested `json:"next"`
	}

	tests := []struct {
		name      string
		setup     func() *Nested
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "chain of 100 nested levels should work",
			setup: func() *Nested {
				root := &Nested{Value: "test"}
				current := root
				for range 100 {
					current.Next = &Nested{Value: "test"}
					current = current.Next
				}

				return root
			},
			wantError: false,
		},
		{
			name: "missing value at depth 50 should error",
			setup: func() *Nested {
				root := &Nested{Value: "test"}
				current := root
				for range 50 {
					current.Next = &Nested{Value: "test"}
					current = current.Next
				}
				current.Next = &Nested{} // Missing value

				return root
			},
			wantError: true,
		},
		{
			name: "chain of 10 nested levels should work",
			setup: func() *Nested {
				root := &Nested{Value: "test"}
				current := root
				for range 10 {
					current.Next = &Nested{Value: "test"}
					current = current.Next
				}

				return root
			},
			wantError: false,
		},
		{
			name: "single node should work",
			setup: func() *Nested {
				return &Nested{Value: "test"}
			},
			wantError: false,
		},
		{
			name: "single node with missing value should error",
			setup: func() *Nested {
				return &Nested{} // Missing value
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := tt.setup()
			err := Validate(t.Context(), root, WithStrategy(StrategyTags))
			if tt.wantError {
				require.Error(t, err, "expected validation error for missing required field")
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
