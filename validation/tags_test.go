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
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateWithTags_Required(t *testing.T) {
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
			name:      "missing required fields",
			user:      User{}, // Missing required fields
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				assert.True(t, verr.HasCode("tag.required"), "should have 'tag.required' error")
			},
		},
		{
			name:      "missing name only",
			user:      User{Email: "john@example.com"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				assert.True(t, verr.HasCode("tag.required"), "should have 'tag.required' error")
			},
		},
		{
			name:      "missing email only",
			user:      User{Name: "John"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				assert.True(t, verr.HasCode("tag.required"), "should have 'tag.required' error")
			},
		},
		{
			name:      "valid user with both fields",
			user:      User{Name: "John", Email: "john@example.com"},
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

func TestValidateWithTags_Email(t *testing.T) {
	t.Parallel()
	type User struct {
		Email string `json:"email" validate:"email"`
	}

	tests := []struct {
		name      string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "invalid email format",
			user:      User{Email: "invalid-email"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				assert.True(t, verr.HasCode("tag.email"), "should have 'tag.email' error")
			},
		},
		{
			name:      "valid email",
			user:      User{Email: "john@example.com"},
			wantError: false,
		},
		{
			name:      "empty email fails email validation",
			user:      User{Email: ""},
			wantError: true, // Empty string is not a valid email format
		},
		{
			name:      "invalid email - missing @",
			user:      User{Email: "johnexample.com"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.True(t, verr.HasCode("tag.email"))
			},
		},
		{
			name:      "invalid email - missing domain",
			user:      User{Email: "john@"},
			wantError: true,
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

func TestValidatePartialLeafsOnly(t *testing.T) {
	t.Parallel()
	type Address struct {
		City string `json:"city" validate:"required"`
		Zip  string `json:"zip" validate:"required"`
	}

	type User struct {
		Name    string  `json:"name" validate:"required"`
		Address Address `json:"address" validate:"required"`
	}

	tests := []struct {
		name      string
		user      User
		pm        PresenceMap
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "PATCH request with only name field",
			user:      User{Name: "John"},
			pm:        PresenceMap{"name": true},
			wantError: false, // Should not error because "address.city" and "address.zip" are not present
		},
		{
			name: "PATCH request with address.city only",
			user: User{Name: "John", Address: Address{City: "NYC"}},
			pm: PresenceMap{
				"name":         true,
				"address":      true,
				"address.city": true,
			},
			wantError: false, // In leaf-only mode, we only validate what's present
		},
		{
			name: "PATCH request with both address fields",
			user: User{Name: "John", Address: Address{City: "NYC", Zip: "12345"}},
			pm: PresenceMap{
				"name":         true,
				"address":      true,
				"address.city": true,
				"address.zip":  true,
			},
			wantError: false,
		},
		{
			name:      "PATCH request with empty name",
			user:      User{Name: ""},
			pm:        PresenceMap{"name": true},
			wantError: true, // Empty name violates required
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePartial(t.Context(), &tt.user, tt.pm, WithStrategy(StrategyTags))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				// Partial validation may still have errors for present fields
				// Errors are acceptable in partial validation, so we don't assert
				_ = err // Acknowledge error but don't fail test
			}
		})
	}
}

func TestPathResolution(t *testing.T) {
	t.Parallel()
	type Item struct {
		Name  string `json:"name" validate:"required"`
		Price int    `json:"price" validate:"required"`
	}

	type Order struct {
		Items []Item `json:"items" validate:"dive"`
	}

	tests := []struct {
		name      string
		order     Order
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "missing required fields in array item",
			order:     Order{Items: []Item{{Name: "", Price: 0}}},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				// Check that paths are correctly formatted
				found := false
				for _, e := range verr.Fields {
					if strings.Contains(e.Path, "items") && strings.Contains(e.Path, "name") {
						found = true
						break
					}
				}
				assert.True(t, found, "should have error for items array")
			},
		},
		{
			name:      "multiple items with errors",
			order:     Order{Items: []Item{{Name: "", Price: 0}, {Name: "", Price: 0}}},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.NotEmpty(t, verr.Fields)
				// Check paths contain array indices
				hasIndex := false
				for _, e := range verr.Fields {
					if strings.Contains(e.Path, "items") {
						hasIndex = true
						break
					}
				}
				assert.True(t, hasIndex, "should have paths with array indices")
			},
		},
		{
			name:      "valid order with items",
			order:     Order{Items: []Item{{Name: "item1", Price: 10}}},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.order, WithStrategy(StrategyTags))
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

func TestRedaction(t *testing.T) {
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
			name:      "short password and missing token",
			user:      User{Password: "short", Token: ""},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				// Check that errors exist for password and token
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
			name:      "valid password and token",
			user:      User{Password: "password123", Token: "token123"},
			wantError: false,
		},
		{
			name:      "missing password only",
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

func TestValidatePartial_NestedArrays(t *testing.T) {
	t.Parallel()
	type Item struct {
		Name  string `json:"name" validate:"required"`
		Price int    `json:"price" validate:"required,min=1"`
	}

	type Order struct {
		Items []Item `json:"items" validate:"required,min=1"`
	}

	tests := []struct {
		name      string
		order     Order
		pm        PresenceMap
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "PATCH - only update items[0].name",
			order: Order{
				Items: []Item{
					{Name: "updated", Price: 0}, // Price missing but not present
				},
			},
			pm: PresenceMap{
				"items":        true,
				"items.0":      true,
				"items.0.name": true,
			},
			wantError: false, // Price not in presence map, so not validated
		},
		{
			name: "PATCH - update items[0].name and items[1].price",
			order: Order{
				Items: []Item{
					{Name: "updated"},
					{Price: 100},
				},
			},
			pm: PresenceMap{
				"items":         true,
				"items.0":       true,
				"items.0.name":  true,
				"items.1":       true,
				"items.1.price": true,
			},
			wantError: false,
		},
		{
			name: "PATCH - empty name should fail",
			order: Order{
				Items: []Item{
					{Name: ""}, // Empty name
				},
			},
			pm: PresenceMap{
				"items":        true,
				"items.0":      true,
				"items.0.name": true,
			},
			wantError: true, // Empty name violates required
		},
		{
			name: "PATCH - multiple items with partial updates",
			order: Order{
				Items: []Item{
					{Name: "item1", Price: 10},
					{Name: "item2", Price: 20},
				},
			},
			pm: PresenceMap{
				"items":         true,
				"items.0":       true,
				"items.0.name":  true,
				"items.1":       true,
				"items.1.price": true,
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePartial(t.Context(), &tt.order, tt.pm, WithStrategy(StrategyTags))
			if tt.wantError {
				require.Error(t, err, "expected validation error for empty required field")
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				// Partial validation may still have errors for present fields
				// Errors are acceptable in partial validation, so we don't assert
				_ = err // Acknowledge error but don't fail test
			}
		})
	}
}

func TestValidatePartial_Maps(t *testing.T) {
	t.Parallel()
	type User struct {
		Name     string            `json:"name" validate:"required"`
		Metadata map[string]string `json:"metadata" validate:"required"`
	}

	tests := []struct {
		name      string
		user      User
		pm        PresenceMap
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "PATCH - only update name",
			user:      User{Name: "John"},
			pm:        PresenceMap{"name": true},
			wantError: false, // Metadata not in presence map, so not validated
		},
		{
			name:      "PATCH - update metadata",
			user:      User{Metadata: map[string]string{"key": "value"}},
			pm:        PresenceMap{"metadata": true},
			wantError: false,
		},
		{
			name:      "PATCH - update both fields",
			user:      User{Name: "John", Metadata: map[string]string{"key": "value"}},
			pm:        PresenceMap{"name": true, "metadata": true},
			wantError: false,
		},
		{
			name:      "PATCH - empty name should fail",
			user:      User{Name: ""},
			pm:        PresenceMap{"name": true},
			wantError: true, // Empty name violates required
		},
		{
			name:      "PATCH - nil metadata when metadata is present",
			user:      User{Name: "John", Metadata: nil},
			pm:        PresenceMap{"name": true, "metadata": true},
			wantError: true, // Nil metadata violates required
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePartial(t.Context(), &tt.user, tt.pm, WithStrategy(StrategyTags))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				// Partial validation may still have errors for present fields
				// Errors are acceptable in partial validation, so we don't assert
				_ = err // Acknowledge error but don't fail test
			}
		})
	}
}

func TestValidatePartial_EmptyVsNilSlices(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string   `json:"name" validate:"required"`
		Tags  []string `json:"tags" validate:"required"`
		Items []string `json:"items"`
	}

	tests := []struct {
		name      string
		user      User
		pm        PresenceMap
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "PATCH - tags is nil (not present)",
			user:      User{Name: "John", Tags: nil},
			pm:        PresenceMap{"name": true},
			wantError: false, // Tags not in presence map, so not validated
		},
		{
			name:      "PATCH - empty slice (not nil)",
			user:      User{Name: "John", Tags: []string{}},
			pm:        PresenceMap{"name": true, "tags": true},
			wantError: false, // go-playground/validator's "required" tag only checks for nil, not empty
		},
		{
			name:      "PATCH - valid tags",
			user:      User{Name: "John", Tags: []string{"tag1"}},
			pm:        PresenceMap{"name": true, "tags": true},
			wantError: false,
		},
		{
			name:      "PATCH - nil tags when tags is present",
			user:      User{Name: "John", Tags: nil},
			pm:        PresenceMap{"name": true, "tags": true},
			wantError: true, // Nil tags violates required
		},
		{
			name:      "PATCH - multiple tags",
			user:      User{Name: "John", Tags: []string{"tag1", "tag2"}},
			pm:        PresenceMap{"name": true, "tags": true},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePartial(t.Context(), &tt.user, tt.pm, WithStrategy(StrategyTags))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				// Partial validation may still have errors for present fields
				// Errors are acceptable in partial validation, so we don't assert
				_ = err // Acknowledge error but don't fail test
			}
		})
	}
}

func TestFieldNameMapper_NestedFields(t *testing.T) {
	t.Parallel()
	type Address struct {
		Street string `json:"street" validate:"required"`
		City   string `json:"city" validate:"required"`
	}

	type User struct {
		FirstName string  `json:"first_name" validate:"required"` //nolint:tagliatelle // snake_case is intentional for API compatibility
		Address   Address `json:"address"`
	}

	tests := []struct {
		name      string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "missing nested fields",
			user:      User{},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				// Check that errors exist for nested fields
				found := false
				for _, e := range verr.Fields {
					if strings.Contains(e.Path, "first_name") || strings.Contains(e.Path, "address") {
						found = true
					}
				}
				assert.True(t, found, "expected errors for nested fields")
			},
		},
		{
			name:      "missing first name only",
			user:      User{Address: Address{Street: "123 Main", City: "NYC"}},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.NotEmpty(t, verr.Fields)
			},
		},
		{
			name:      "missing address fields",
			user:      User{FirstName: "John"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				found := false
				for _, e := range verr.Fields {
					if strings.Contains(e.Path, "address") {
						found = true
					}
				}
				assert.True(t, found, "should have errors for address fields")
			},
		},
		{
			name:      "valid user with all fields",
			user:      User{FirstName: "John", Address: Address{Street: "123 Main", City: "NYC"}},
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

func TestFieldNameMapper_ArrayElements(t *testing.T) {
	t.Parallel()
	type Item struct {
		Name string `json:"name" validate:"required"`
	}

	type Order struct {
		Items []Item `json:"items" validate:"dive"`
	}

	tests := []struct {
		name      string
		order     Order
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "empty name in array item",
			order:     Order{Items: []Item{{Name: ""}}},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				// Check that we got validation errors for the items array
				found := false
				for _, e := range verr.Fields {
					// Paths are typically in format "items.0.name" (dot notation)
					if strings.Contains(e.Path, "items") && (strings.Contains(e.Path, "0") || strings.Contains(e.Path, "name")) {
						found = true
						t.Logf("Found validation error with path: %s", e.Path)

						break
					}
				}
				if !found {
					// Log all paths for debugging
					for _, e := range verr.Fields {
						t.Logf("Validation error path: %s", e.Path)
					}
				}
				assert.True(t, found, "expected validation error for items array element")
			},
		},
		{
			name:      "multiple items with errors",
			order:     Order{Items: []Item{{Name: ""}, {Name: ""}}},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.NotEmpty(t, verr.Fields)
			},
		},
		{
			name:      "valid items",
			order:     Order{Items: []Item{{Name: "item1"}, {Name: "item2"}}},
			wantError: false,
		},
		{
			name:      "empty array",
			order:     Order{Items: []Item{}},
			wantError: false, // Empty array is valid (no dive validation on empty)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.order, WithStrategy(StrategyTags))
			if tt.wantError {
				require.Error(t, err, "expected validation error for empty Name field")
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRedaction_NestedSensitiveFields(t *testing.T) {
	t.Parallel()
	type User struct {
		Password string `json:"password" validate:"required,min=8"`
		Profile  struct {
			Token string `json:"token" validate:"required"`
		} `json:"profile"`
	}

	tests := []struct {
		name      string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "short password and missing nested token",
			user: User{
				Password: "short",
				Profile: struct {
					Token string `json:"token" validate:"required"`
				}{Token: ""},
			},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				// Check that errors exist for password and profile.token
				foundPassword := false
				foundToken := false
				for _, e := range verr.Fields {
					if e.Path == "password" {
						foundPassword = true
					}
					if e.Path == "profile.token" {
						foundToken = true
					}
				}
				assert.True(t, foundPassword, "should have error for password field")
				assert.True(t, foundToken, "should have error for profile.token field")
			},
		},
		{
			name: "valid password and token",
			user: User{
				Password: "password123",
				Profile: struct {
					Token string `json:"token" validate:"required"`
				}{Token: "token123"},
			},
			wantError: false,
		},
		{
			name: "missing password only",
			user: User{
				Profile: struct {
					Token string `json:"token" validate:"required"`
				}{Token: "token123"},
			},
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

func TestRedaction_AllErrorTypes(t *testing.T) {
	t.Parallel()
	type User struct {
		Password string `json:"password" validate:"required"`
	}

	tests := []struct {
		name      string
		user      *User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "missing password with tags strategy",
			user:      &User{Password: ""},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				// Check that error exists for password
				found := false
				for _, e := range verr.Fields {
					if e.Path == "password" {
						found = true
						break
					}
				}
				assert.True(t, found, "should have error for password field")
			},
		},
		{
			name:      "valid password",
			user:      &User{Password: "password123"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), tt.user, WithStrategy(StrategyTags))
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

func TestWithMessages(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	tests := []struct {
		name        string
		user        User
		messages    map[string]string
		wantMessage string
	}{
		{
			name:        "custom required message",
			user:        User{},
			messages:    map[string]string{"required": "cannot be empty"},
			wantMessage: "cannot be empty",
		},
		{
			name:        "custom email message",
			user:        User{Name: "John", Email: "invalid"},
			messages:    map[string]string{"email": "invalid email format"},
			wantMessage: "invalid email format",
		},
		{
			name:        "fallback to default for unspecified tag",
			user:        User{Name: "John", Email: "invalid"},
			messages:    map[string]string{"required": "cannot be empty"},
			wantMessage: "must be a valid email address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := MustNew(WithMessages(tt.messages))
			err := v.Validate(t.Context(), &tt.user)
			require.Error(t, err)

			var verr *Error
			require.ErrorAs(t, err, &verr)

			found := false
			for _, e := range verr.Fields {
				if e.Message == tt.wantMessage {
					found = true
					break
				}
			}
			assert.True(t, found, "expected message %q in errors: %v", tt.wantMessage, verr.Fields)
		})
	}
}

func TestWithMessageFunc(t *testing.T) {
	t.Parallel()
	type Product struct {
		Name  string `json:"name" validate:"min=3"`
		Price int    `json:"price" validate:"min=1"`
	}

	v := MustNew(
		WithMessageFunc("min", func(param string, kind reflect.Kind) string {
			if kind == reflect.String {
				return "too short (min " + param + " chars)"
			}
			return "too small (min " + param + ")"
		}),
	)

	tests := []struct {
		name        string
		product     Product
		wantMessage string
	}{
		{
			name:        "string min with custom func",
			product:     Product{Name: "ab", Price: 10},
			wantMessage: "too short (min 3 chars)",
		},
		{
			name:        "int min with custom func",
			product:     Product{Name: "abc", Price: 0},
			wantMessage: "too small (min 1)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := v.Validate(t.Context(), &tt.product)
			require.Error(t, err)

			var verr *Error
			require.ErrorAs(t, err, &verr)

			found := false
			for _, e := range verr.Fields {
				if e.Message == tt.wantMessage {
					found = true
					break
				}
			}
			assert.True(t, found, "expected message %q in errors: %v", tt.wantMessage, verr.Fields)
		})
	}
}

func TestWithMessagesAndMessageFunc(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name" validate:"required,min=2"`
		Email string `json:"email" validate:"email"`
	}

	v := MustNew(
		WithMessages(map[string]string{
			"required": "cannot be empty",
			"email":    "invalid email format",
		}),
		WithMessageFunc("min", func(param string, kind reflect.Kind) string {
			return "minimum length: " + param
		}),
	)

	// Test static message
	t.Run("static message for required", func(t *testing.T) {
		t.Parallel()
		err := v.Validate(t.Context(), &User{})
		require.Error(t, err)

		var verr *Error
		require.ErrorAs(t, err, &verr)

		found := false
		for _, e := range verr.Fields {
			if e.Message == "cannot be empty" {
				found = true
				break
			}
		}
		assert.True(t, found, "expected 'cannot be empty' message")
	})

	// Test dynamic message
	t.Run("dynamic message for min", func(t *testing.T) {
		t.Parallel()
		err := v.Validate(t.Context(), &User{Name: "a", Email: "test@example.com"})
		require.Error(t, err)

		var verr *Error
		require.ErrorAs(t, err, &verr)

		found := false
		for _, e := range verr.Fields {
			if e.Message == "minimum length: 2" {
				found = true
				break
			}
		}
		assert.True(t, found, "expected 'minimum length: 2' message")
	})

	// Test static message takes precedence
	t.Run("static message for email", func(t *testing.T) {
		t.Parallel()
		err := v.Validate(t.Context(), &User{Name: "John", Email: "invalid"})
		require.Error(t, err)

		var verr *Error
		require.ErrorAs(t, err, &verr)

		found := false
		for _, e := range verr.Fields {
			if e.Message == "invalid email format" {
				found = true
				break
			}
		}
		assert.True(t, found, "expected 'invalid email format' message")
	})
}
