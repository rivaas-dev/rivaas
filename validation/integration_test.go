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

//go:build integration

package validation_test

import (
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/validation"
)

// Integration tests for the validation package.
// These tests verify end-to-end workflows and component interactions.

//nolint:tparallel // False positive: t.Parallel() is called at both top level and in subtests
func TestIntegration_FullValidationWorkflow(t *testing.T) {
	t.Parallel()

	type Address struct {
		Street string `json:"street" validate:"required"`
		City   string `json:"city" validate:"required"`
		Zip    string `json:"zip" validate:"required"`
	}

	type User struct {
		Name    string  `json:"name" validate:"required"`
		Email   string  `json:"email" validate:"required,email"`
		Age     int     `json:"age" validate:"min=18,max=120"`
		Address Address `json:"address"`
	}

	tests := []struct {
		name       string
		jsonInput  string
		wantError  bool
		errorCount int
		checkErr   func(t *testing.T, err error)
	}{
		{
			name: "valid complete user",
			jsonInput: `{
				"name": "John Doe",
				"email": "john@example.com",
				"age": 30,
				"address": {
					"street": "123 Main St",
					"city": "New York",
					"zip": "10001"
				}
			}`,
			wantError: false,
		},
		{
			name: "missing required fields",
			jsonInput: `{
				"name": "",
				"email": "invalid",
				"age": 15
			}`,
			wantError:  true,
			errorCount: 3, // name required, email invalid, age below min
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *validation.Error
				require.ErrorAs(t, err, &verr)
				assert.GreaterOrEqual(t, len(verr.Fields), 3)
			},
		},
		{
			name: "nested validation errors",
			jsonInput: `{
				"name": "John",
				"email": "john@example.com",
				"age": 25,
				"address": {
					"street": "",
					"city": "",
					"zip": ""
				}
			}`,
			wantError:  true,
			errorCount: 3, // all address fields missing
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *validation.Error
				require.ErrorAs(t, err, &verr)
				// Should have errors for nested address fields
				hasAddressErrors := false
				for _, fe := range verr.Fields {
					if fe.Path == "address.street" || fe.Path == "address.city" || fe.Path == "address.zip" {
						hasAddressErrors = true
						break
					}
				}
				assert.True(t, hasAddressErrors, "should have address field errors")
			},
		},
		{
			name: "age boundary - exactly 18",
			jsonInput: `{
				"name": "Young User",
				"email": "young@example.com",
				"age": 18,
				"address": {"street": "1 St", "city": "City", "zip": "12345"}
			}`,
			wantError: false,
		},
		{
			name: "age boundary - exactly 120",
			jsonInput: `{
				"name": "Old User",
				"email": "old@example.com",
				"age": 120,
				"address": {"street": "1 St", "city": "City", "zip": "12345"}
			}`,
			wantError: false,
		},
		{
			name: "age boundary - above 120",
			jsonInput: `{
				"name": "Too Old",
				"email": "tooold@example.com",
				"age": 121,
				"address": {"street": "1 St", "city": "City", "zip": "12345"}
			}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Step 1: Parse JSON input
			var user User
			err := json.Unmarshal([]byte(tt.jsonInput), &user)
			require.NoError(t, err, "JSON parsing should succeed")

			// Step 2: Validate the parsed struct
			ctx := t.Context()
			err = validation.Validate(ctx, &user)

			// Step 3: Verify validation result
			if tt.wantError {
				require.Error(t, err)
				require.ErrorIs(t, err, validation.ErrValidation)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

//nolint:tparallel // False positive: t.Parallel() is called at both top level and in subtests
func TestIntegration_PartialValidationWorkflow(t *testing.T) {
	t.Parallel()

	type User struct {
		Name    string `json:"name" validate:"required"`
		Email   string `json:"email" validate:"required,email"`
		Address string `json:"address" validate:"required"`
	}

	tests := []struct {
		name        string
		jsonPatch   string
		existingVal User
		wantError   bool
		checkErr    func(t *testing.T, err error)
	}{
		{
			name:        "PATCH only name - valid",
			jsonPatch:   `{"name": "Updated Name"}`,
			existingVal: User{Name: "Old Name", Email: "old@example.com", Address: "123 St"},
			wantError:   false,
		},
		{
			name:        "PATCH only email - valid",
			jsonPatch:   `{"email": "new@example.com"}`,
			existingVal: User{Name: "Name", Email: "old@example.com", Address: "123 St"},
			wantError:   false,
		},
		{
			name:        "PATCH email with invalid format",
			jsonPatch:   `{"email": "invalid-email"}`,
			existingVal: User{Name: "Name", Email: "old@example.com", Address: "123 St"},
			wantError:   true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *validation.Error
				require.ErrorAs(t, err, &verr)
				assert.True(t, verr.Has("email"))
			},
		},
		{
			name:        "PATCH name with empty string",
			jsonPatch:   `{"name": ""}`,
			existingVal: User{Name: "Old Name", Email: "email@example.com", Address: "123 St"},
			wantError:   true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *validation.Error
				require.ErrorAs(t, err, &verr)
				assert.True(t, verr.Has("name"))
			},
		},
		{
			name:        "PATCH multiple fields - all valid",
			jsonPatch:   `{"name": "New Name", "email": "new@example.com"}`,
			existingVal: User{Name: "Old", Email: "old@example.com", Address: "123 St"},
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Step 1: Compute presence map from JSON patch
			presence, err := validation.ComputePresence([]byte(tt.jsonPatch))
			require.NoError(t, err)

			// Step 2: Apply patch to existing value (simulated)
			var patch map[string]any
			err = json.Unmarshal([]byte(tt.jsonPatch), &patch)
			require.NoError(t, err)

			user := tt.existingVal
			if name, ok := patch["name"].(string); ok {
				user.Name = name
			}
			if email, ok := patch["email"].(string); ok {
				user.Email = email
			}
			if address, ok := patch["address"].(string); ok {
				user.Address = address
			}

			// Step 3: Validate only present fields
			ctx := t.Context()
			err = validation.ValidatePartial(ctx, &user, presence)

			// Step 4: Verify result
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

//nolint:tparallel // False positive: t.Parallel() is called at both top level and in subtests
func TestIntegration_ValidatorInstance(t *testing.T) {
	t.Parallel()

	type SensitiveUser struct {
		Username string `json:"username" validate:"required"`
		Password string `json:"password" validate:"required,min=8"`
		APIKey   string `json:"api_key" validate:"required"` //nolint:tagliatelle // snake_case is intentional for API compatibility
	}

	// Create validator with custom configuration
	validator, err := validation.New(
		validation.WithMaxErrors(5),
		validation.WithRedactor(func(path string) bool {
			return path == "password" || path == "api_key"
		}),
	)
	require.NoError(t, err)

	tests := []struct {
		name      string
		user      SensitiveUser
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "valid user",
			user: SensitiveUser{
				Username: "testuser",
				Password: "securepassword123",
				APIKey:   "api-key-12345",
			},
			wantError: false,
		},
		{
			name: "short password - should be redacted in error",
			user: SensitiveUser{
				Username: "testuser",
				Password: "short",
				APIKey:   "api-key-12345",
			},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *validation.Error
				require.ErrorAs(t, err, &verr)
				assert.True(t, verr.Has("password"))
				// Check that the error value is redacted
				fe := verr.GetField("password")
				require.NotNil(t, fe)
				if fe.Meta != nil {
					if val, ok := fe.Meta["value"]; ok {
						assert.Equal(t, "***REDACTED***", val)
					}
				}
			},
		},
		{
			name:      "missing all fields",
			user:      SensitiveUser{},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *validation.Error
				require.ErrorAs(t, err, &verr)
				assert.GreaterOrEqual(t, len(verr.Fields), 3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			validateErr := validator.Validate(ctx, &tt.user)

			if tt.wantError {
				require.Error(t, validateErr)
				if tt.checkErr != nil {
					tt.checkErr(t, validateErr)
				}
			} else {
				assert.NoError(t, validateErr)
			}
		})
	}
}

func TestIntegration_ConcurrentValidation(t *testing.T) {
	t.Parallel()

	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	const (
		numGoroutines           = 50
		validationsPerGoroutine = 100
	)

	var wg sync.WaitGroup
	errChan := make(chan error, numGoroutines*validationsPerGoroutine)

	// Test concurrent validations don't interfere with each other
	for i := range numGoroutines {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()

			for j := range validationsPerGoroutine {
				// Alternate between valid and invalid users
				var user User
				if j%2 == 0 {
					user = User{Name: "Valid User", Email: "valid@example.com"}
				} else {
					user = User{Name: "", Email: "invalid"}
				}

				ctx := t.Context()
				err := validation.Validate(ctx, &user)

				// Valid users should pass, invalid should fail
				if j%2 == 0 && err != nil {
					errChan <- err
				}
				if j%2 == 1 && err == nil {
					errChan <- errors.New("expected validation error for invalid user")
				}
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Collect any unexpected errors
	var unexpectedErrors []error
	for err := range errChan {
		unexpectedErrors = append(unexpectedErrors, err)
	}

	assert.Empty(t, unexpectedErrors, "no unexpected errors during concurrent validation")
}

//nolint:tparallel // False positive: t.Parallel() is called at both top level and in subtests
func TestIntegration_JSONSchemaWithCustomSchema(t *testing.T) {
	t.Parallel()

	type Product struct {
		ID    string  `json:"id"`
		Name  string  `json:"name"`
		Price float64 `json:"price"`
		Stock int     `json:"stock"`
	}

	schema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"id": {"type": "string", "pattern": "^[A-Z]{2}[0-9]{4}$"},
			"name": {"type": "string", "minLength": 1, "maxLength": 100},
			"price": {"type": "number", "minimum": 0, "exclusiveMinimum": 0},
			"stock": {"type": "integer", "minimum": 0}
		},
		"required": ["id", "name", "price"]
	}`

	tests := []struct {
		name      string
		product   Product
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "valid product",
			product: Product{
				ID:    "AB1234",
				Name:  "Test Product",
				Price: 29.99,
				Stock: 100,
			},
			wantError: false,
		},
		{
			name: "invalid ID pattern",
			product: Product{
				ID:    "invalid-id",
				Name:  "Test Product",
				Price: 29.99,
			},
			wantError: true,
		},
		{
			name: "zero price - should fail",
			product: Product{
				ID:    "AB1234",
				Name:  "Test Product",
				Price: 0,
			},
			wantError: true,
		},
		{
			name: "negative stock",
			product: Product{
				ID:    "AB1234",
				Name:  "Test Product",
				Price: 10.00,
				Stock: -5,
			},
			wantError: true,
		},
		{
			name: "empty name",
			product: Product{
				ID:    "AB1234",
				Name:  "",
				Price: 10.00,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			err := validation.Validate(ctx, &tt.product,
				validation.WithStrategy(validation.StrategyJSONSchema),
				validation.WithCustomSchema("product-schema-"+tt.name, schema),
			)

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

func TestIntegration_ErrorChaining(t *testing.T) {
	t.Parallel()

	type User struct {
		Name string `json:"name" validate:"required"`
	}

	ctx := t.Context()
	user := &User{Name: ""}

	err := validation.Validate(ctx, user)
	require.Error(t, err)

	// Test errors.Is works through the chain
	require.ErrorIs(t, err, validation.ErrValidation)

	// Test errors.As works
	var verr *validation.Error
	require.ErrorAs(t, err, &verr)
	assert.NotEmpty(t, verr.Fields)

	// Test Unwrap returns ErrValidation
	unwrapped := verr.Unwrap()
	assert.ErrorIs(t, unwrapped, validation.ErrValidation)
}

//nolint:tparallel // False positive: t.Parallel() is called at both top level and in subtests
func TestIntegration_MaxErrorsTruncation(t *testing.T) {
	t.Parallel()

	type ManyFields struct {
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
	}

	tests := []struct {
		name          string
		maxErrors     int
		wantMaxFields int
		wantTruncated bool
	}{
		{
			name:          "limit to 3 errors",
			maxErrors:     3,
			wantMaxFields: 3,
			wantTruncated: true,
		},
		{
			name:          "limit to 5 errors",
			maxErrors:     5,
			wantMaxFields: 5,
			wantTruncated: true,
		},
		{
			name:          "no limit (0)",
			maxErrors:     0,
			wantMaxFields: 10,
			wantTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()
			data := &ManyFields{} // All fields empty

			err := validation.Validate(ctx, data, validation.WithMaxErrors(tt.maxErrors))
			require.Error(t, err)

			var verr *validation.Error
			require.ErrorAs(t, err, &verr)

			assert.LessOrEqual(t, len(verr.Fields), tt.wantMaxFields)
			if tt.wantTruncated {
				assert.True(t, verr.Truncated, "should be marked as truncated")
			}
		})
	}
}

func TestIntegration_ComputePresenceNestedJSON(t *testing.T) {
	t.Parallel()


	tests := []struct {
		name           string
		jsonInput      string
		expectedLeaves []string
		checkPresence  func(t *testing.T, pm validation.PresenceMap)
	}{
		{
			name: "flat object",
			jsonInput: `{
				"name": "John",
				"email": "john@example.com"
			}`,
			expectedLeaves: []string{"name", "email"},
		},
		{
			name: "nested object",
			jsonInput: `{
				"user": {
					"name": "John",
					"profile": {
						"bio": "Hello"
					}
				}
			}`,
			expectedLeaves: []string{"user.name", "user.profile.bio"},
			checkPresence: func(t *testing.T, pm validation.PresenceMap) {
				t.Helper()
				assert.True(t, pm.Has("user"))
				assert.True(t, pm.Has("user.name"))
				assert.True(t, pm.Has("user.profile"))
				assert.True(t, pm.Has("user.profile.bio"))
				assert.True(t, pm.HasPrefix("user"))
				assert.True(t, pm.HasPrefix("user.profile"))
			},
		},
		{
			name: "array with objects",
			jsonInput: `{
				"items": [
					{"id": 1, "name": "Item 1"},
					{"id": 2}
				]
			}`,
			checkPresence: func(t *testing.T, pm validation.PresenceMap) {
				t.Helper()
				assert.True(t, pm.Has("items"))
				assert.True(t, pm.Has("items.0"))
				assert.True(t, pm.Has("items.0.id"))
				assert.True(t, pm.Has("items.0.name"))
				assert.True(t, pm.Has("items.1"))
				assert.True(t, pm.Has("items.1.id"))
				assert.False(t, pm.Has("items.1.name"))
			},
		},
		{
			name:      "empty object",
			jsonInput: `{}`,
			checkPresence: func(t *testing.T, pm validation.PresenceMap) {
				t.Helper()
				leaves := pm.LeafPaths()
				assert.Empty(t, leaves)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pm, err := validation.ComputePresence([]byte(tt.jsonInput))
			require.NoError(t, err)

			if tt.expectedLeaves != nil {
				leaves := pm.LeafPaths()
				for _, expected := range tt.expectedLeaves {
					assert.True(t, slices.Contains(leaves, expected), "expected leaf %q not found", expected)
				}
			}

			if tt.checkPresence != nil {
				tt.checkPresence(t, pm)
			}
		})
	}
}
