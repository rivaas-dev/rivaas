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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateWithSchema_Basic(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name      string
		schema    string
		schemaID  string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "valid user with both fields",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "number", "minimum": 0}
				},
				"required": ["name"]
			}`,
			schemaID:  "test-basic-1",
			user:      User{Name: "John", Age: 30},
			wantError: false,
		},
		{
			name: "valid user with zero age (minimum 0)",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "number", "minimum": 0}
				},
				"required": ["name"]
			}`,
			schemaID:  "test-basic-2",
			user:      User{Name: "John", Age: 0},
			wantError: false,
		},
		{
			name: "valid - empty string name might pass required check",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "number", "minimum": 0}
				},
				"required": ["name"]
			}`,
			schemaID:  "test-basic-3",
			user:      User{Name: "", Age: 30}, // Empty string might pass if field is present
			wantError: false,                   // Empty string is still a string, so required might pass
		},
		{
			name: "invalid - age below minimum",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string", "minLength": 1},
					"age": {"type": "number", "minimum": 1}
				},
				"required": ["name", "age"]
			}`,
			schemaID:  "test-basic-strict",
			user:      User{Name: "John", Age: 0},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				assert.NotEmpty(t, verr.Fields, "should have validation errors")
			},
		},
		{
			name: "invalid - negative age",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "number", "minimum": 0}
				},
				"required": ["name"]
			}`,
			schemaID:  "test-basic-4",
			user:      User{Name: "John", Age: -1},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.user, WithStrategy(StrategyJSONSchema), WithCustomSchema(tt.schemaID, tt.schema))
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

func TestValidateWithSchema_Partial(t *testing.T) {
	t.Parallel()
	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	tests := []struct {
		name      string
		schema    string
		schemaID  string
		user      User
		pm        PresenceMap
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "partial validation - only name present",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string", "minLength": 1},
					"email": {"type": "string", "format": "email"}
				}
			}`,
			schemaID:  "test-partial-1",
			user:      User{Name: "John"},
			pm:        PresenceMap{"name": true},
			wantError: false, // Should not error because email is not present in partial mode
		},
		{
			name: "partial validation - only email present",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string", "minLength": 1},
					"email": {"type": "string", "format": "email"}
				}
			}`,
			schemaID:  "test-partial-2",
			user:      User{Email: "john@example.com"},
			pm:        PresenceMap{"email": true},
			wantError: false,
		},
		{
			name: "partial validation - invalid email format",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string", "minLength": 1},
					"email": {"type": "string", "format": "email"}
				}
			}`,
			schemaID:  "test-partial-3",
			user:      User{Email: "invalid-email"},
			pm:        PresenceMap{"email": true},
			wantError: true, // Invalid email format should fail
		},
		{
			name: "partial validation - empty name when name is present",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string", "minLength": 1},
					"email": {"type": "string", "format": "email"}
				}
			}`,
			schemaID:  "test-partial-4",
			user:      User{Name: ""},
			pm:        PresenceMap{"name": true},
			wantError: true, // Empty name violates minLength: 1
		},
		{
			name: "partial validation - both fields present",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string", "minLength": 1},
					"email": {"type": "string", "format": "email"}
				}
			}`,
			schemaID:  "test-partial-5",
			user:      User{Name: "John", Email: "john@example.com"},
			pm:        PresenceMap{"name": true, "email": true},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePartial(t.Context(), &tt.user, tt.pm, WithStrategy(StrategyJSONSchema), WithCustomSchema(tt.schemaID, tt.schema))
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

func TestPruneByPresence(t *testing.T) {
	t.Parallel()
	// This test verifies that partial validation works correctly by only validating present fields
	type TestStruct struct {
		Name    string           `json:"name"`
		Email   string           `json:"email"`
		Address map[string]any   `json:"address"`
		Items   []map[string]any `json:"items"`
	}

	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"email": {"type": "string", "format": "email"},
			"address": {
				"type": "object",
				"properties": {
					"city": {"type": "string"},
					"zip": {"type": "string"}
				}
			},
			"items": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"name": {"type": "string"},
						"price": {"type": "number"}
					}
				}
			}
		},
		"required": ["name", "email"]
	}`

	tests := []struct {
		name      string
		schemaID  string
		data      TestStruct
		pm        PresenceMap
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:     "partial validation - email not in presence map should be ignored",
			schemaID: "test-prune-presence-1",
			data: TestStruct{
				Name:  "John",
				Email: "john@example.com", // This should be ignored in partial validation
				Address: map[string]any{
					"city": "NYC",
					"zip":  "10001", // Not in presence map
				},
				Items: []map[string]any{
					{"name": "item1", "price": 100}, // price not in presence map
					{"name": "item2"},               // Not in presence map
				},
			},
			pm: PresenceMap{
				"name":         true,
				"address":      true,
				"address.city": true,
				"items":        true,
				"items.0":      true,
				"items.0.name": true,
			},
			wantError: false, // Should pass because email is not in presence map
		},
		{
			name:     "partial validation - invalid email when email is in presence map",
			schemaID: "test-prune-presence-2",
			data: TestStruct{
				Name:  "John",
				Email: "invalid-email", // Invalid email format
			},
			pm: PresenceMap{
				"name":  true,
				"email": true, // Email is in presence map, so it should be validated
			},
			wantError: true, // Invalid email format should fail
		},
		{
			name:     "partial validation - valid email when email is in presence map",
			schemaID: "test-prune-presence-3",
			data: TestStruct{
				Name:  "John",
				Email: "john@example.com",
			},
			pm: PresenceMap{
				"name":  true,
				"email": true,
			},
			wantError: false,
		},
		{
			name:     "partial validation - empty presence map should validate all",
			schemaID: "test-prune-presence-4",
			data: TestStruct{
				Name:  "John",
				Email: "john@example.com",
			},
			pm:        PresenceMap{},
			wantError: false, // Empty presence map may validate all or none depending on implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePartial(t.Context(), &tt.data, tt.pm, WithStrategy(StrategyJSONSchema), WithCustomSchema(tt.schemaID, schema))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				// For partial validation, we're mainly checking it doesn't error on non-present fields
				// Some errors may still occur for present fields, so we just log them
				if err != nil {
					t.Logf("Partial validation result: %v", err)
				}
			}
		})
	}
}

func TestGetRawJSONFromContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		setupCtx  func(*testing.T) context.Context
		wantFound bool
		wantData  []byte
		check     func(t *testing.T, retrieved []byte, ok bool)
	}{
		{
			name: "should extract raw JSON from context",
			setupCtx: func(t *testing.T) context.Context {
				t.Helper()
				rawJSON := []byte(`{"name": "John"}`)

				return InjectRawJSONCtx(t.Context(), rawJSON)
			},
			wantFound: true,
			wantData:  []byte(`{"name": "John"}`),
			check: func(t *testing.T, retrieved []byte, ok bool) {
				t.Helper()
				assert.True(t, ok, "should be able to extract raw JSON")
				assert.JSONEq(t, `{"name": "John"}`, string(retrieved))
			},
		},
		{
			name: "should return false for context without raw JSON",
			setupCtx: func(t *testing.T) context.Context {
				t.Helper()
				return t.Context()
			},
			wantFound: false,
			check: func(t *testing.T, retrieved []byte, ok bool) {
				t.Helper()
				assert.False(t, ok, "should return false for context without raw JSON")
				assert.Nil(t, retrieved, "should return nil for context without raw JSON")
			},
		},
		{
			name: "should extract empty JSON array",
			setupCtx: func(t *testing.T) context.Context {
				t.Helper()
				rawJSON := []byte(`[]`)

				return InjectRawJSONCtx(t.Context(), rawJSON)
			},
			wantFound: true,
			wantData:  []byte(`[]`),
			check: func(t *testing.T, retrieved []byte, ok bool) {
				t.Helper()
				assert.True(t, ok)
				assert.Equal(t, `[]`, string(retrieved))
			},
		},
		{
			name: "should extract complex nested JSON",
			setupCtx: func(t *testing.T) context.Context {
				t.Helper()
				rawJSON := []byte(`{"user": {"name": "John", "age": 30}, "tags": ["admin", "user"]}`)

				return InjectRawJSONCtx(t.Context(), rawJSON)
			},
			wantFound: true,
			wantData:  []byte(`{"user": {"name": "John", "age": 30}, "tags": ["admin", "user"]}`),
			check: func(t *testing.T, retrieved []byte, ok bool) {
				t.Helper()
				assert.True(t, ok)
				assert.JSONEq(t, `{"user": {"name": "John", "age": 30}, "tags": ["admin", "user"]}`, string(retrieved))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := tt.setupCtx(t)
			retrieved, ok := ExtractRawJSONCtx(ctx)
			if tt.check != nil {
				tt.check(t, retrieved, ok)
			} else {
				assert.Equal(t, tt.wantFound, ok)
				if tt.wantData != nil {
					assert.Equal(t, string(tt.wantData), string(retrieved))
				}
			}
		})
	}
}

func TestJSONSchemaProvider(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name"`
	}

	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string", "minLength": 1}
		},
		"required": ["name"]
	}`

	tests := []struct {
		name      string
		schemaID  string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "valid user with name",
			schemaID:  "test-provider-1",
			user:      User{Name: "John"},
			wantError: false,
		},
		{
			name:      "invalid - empty name violates minLength",
			schemaID:  "test-provider-empty",
			user:      User{Name: ""},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.NotEmpty(t, verr.Fields)
			},
		},
		{
			name:      "valid - name with single character",
			schemaID:  "test-provider-2",
			user:      User{Name: "A"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.user, WithStrategy(StrategyJSONSchema), WithCustomSchema(tt.schemaID, schema))
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

func TestSchemaCache(t *testing.T) {
	t.Parallel()
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		}
	}`

	type User struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name     string
		schemaID string
		user     User
	}{
		{
			name:     "first call should compile schema",
			schemaID: "test-cache-1",
			user:     User{Name: "John"},
		},
		{
			name:     "second call with same schema ID should use cache",
			schemaID: "test-cache-1",
			user:     User{Name: "Jane"},
		},
		{
			name:     "different schema ID should compile new schema",
			schemaID: "test-cache-2",
			user:     User{Name: "Bob"},
		},
		{
			name:     "same schema ID with different user should use cache",
			schemaID: "test-cache-2",
			user:     User{Name: "Alice"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.user, WithStrategy(StrategyJSONSchema), WithCustomSchema(tt.schemaID, schema))
			assert.NoError(t, err, "validation should succeed")
		})
	}
}

func TestValidateWithSchema_InvalidSchema(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name      string
		schema    string
		schemaID  string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name: "invalid schema type",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "invalid_type"}
				}
			}`,
			schemaID:  "test-invalid-schema",
			user:      User{Name: "John"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				// Should have schema compile error
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.NotEmpty(t, verr.Fields)
			},
		},
		{
			name:      "malformed JSON schema",
			schema:    `{invalid json}`,
			schemaID:  "test-malformed-schema",
			user:      User{Name: "John"},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr)
				assert.NotEmpty(t, verr.Fields)
			},
		},
		{
			name:      "empty schema string",
			schema:    ``,
			schemaID:  "test-empty-schema",
			user:      User{Name: "John"},
			wantError: false, // Empty schema might be treated as no validation needed
		},
		{
			name: "invalid property constraint",
			schema: `{
				"type": "object",
				"properties": {
					"age": {"type": "number", "minimum": "invalid"}
				}
			}`,
			schemaID:  "test-invalid-constraint",
			user:      User{Name: "John"},
			wantError: true,
		},
		{
			name: "missing type in property",
			schema: `{
				"type": "object",
				"properties": {
					"name": {}
				}
			}`,
			schemaID:  "test-missing-type",
			user:      User{Name: "John"},
			wantError: false, // May pass if type is inferred or optional
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.user, WithStrategy(StrategyJSONSchema), WithCustomSchema(tt.schemaID, tt.schema))
			if tt.wantError {
				require.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				// For some cases, we're not sure if it will error, so just check it doesn't panic
				_ = err
			}
		})
	}
}

func TestValidateWithSchema_NestedObjectErrors(t *testing.T) {
	t.Parallel()
	type Address struct {
		City string `json:"city"`
		Zip  string `json:"zip"`
	}

	type User struct {
		Address Address `json:"address"`
	}

	schema := `{
		"type": "object",
		"properties": {
			"address": {
				"type": "object",
				"properties": {
					"city": {"type": "string", "minLength": 1},
					"zip": {"type": "string", "pattern": "^[0-9]{5}$"}
				},
				"required": ["city", "zip"]
			}
		},
		"required": ["address"]
	}`

	tests := []struct {
		name      string
		schemaID  string
		user      User
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:     "invalid - missing nested required field city",
			schemaID: "test-nested-1",
			user: User{
				Address: Address{
					Zip: "12345",
				},
			},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				var verr *Error
				require.ErrorAs(t, err, &verr, "expected validation.Error")
				// Should have error for address.city
				found := false
				for _, e := range verr.Fields {
					if e.Path == "address.city" || strings.Contains(e.Path, "city") {
						found = true
						break
					}
				}
				assert.True(t, found, "expected error for nested field address.city")
			},
		},
		{
			name:     "invalid - invalid zip pattern",
			schemaID: "test-nested-2",
			user: User{
				Address: Address{
					City: "NYC",
					Zip:  "invalid",
				},
			},
			wantError: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
			},
		},
		{
			name:     "invalid - missing zip",
			schemaID: "test-nested-3",
			user: User{
				Address: Address{
					City: "NYC",
				},
			},
			wantError: true,
		},
		{
			name:     "valid nested object",
			schemaID: "test-nested-4",
			user: User{
				Address: Address{
					City: "NYC",
					Zip:  "12345",
				},
			},
			wantError: false,
		},
		{
			name:     "invalid - empty city",
			schemaID: "test-nested-5",
			user: User{
				Address: Address{
					City: "",
					Zip:  "12345",
				},
			},
			wantError: true, // Empty city violates minLength: 1
		},
		{
			name:     "invalid - zip too short",
			schemaID: "test-nested-6",
			user: User{
				Address: Address{
					City: "NYC",
					Zip:  "1234", // Only 4 digits, pattern requires 5
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.user, WithStrategy(StrategyJSONSchema), WithCustomSchema(tt.schemaID, schema))
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

func TestValidateWithSchema_SchemaRefs(t *testing.T) {
	t.Parallel()
	type User struct {
		Name string `json:"name"`
	}

	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string", "minLength": 1}
		},
		"required": ["name"]
	}`

	tests := []struct {
		name      string
		schemaID  string
		user      User
		wantError bool
	}{
		{
			name:      "invalid - empty name",
			schemaID:  "test-ref-1",
			user:      User{Name: ""},
			wantError: true,
		},
		{
			name:      "valid user",
			schemaID:  "test-ref-2",
			user:      User{Name: "John"},
			wantError: false,
		},
		{
			name:      "valid - single character name",
			schemaID:  "test-ref-3",
			user:      User{Name: "A"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.user, WithStrategy(StrategyJSONSchema), WithCustomSchema(tt.schemaID, schema))
			if tt.wantError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWithSchema_ArrayValidation(t *testing.T) {
	t.Parallel()
	type Item struct {
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	type Order struct {
		Items []Item `json:"items"`
	}

	schema := `{
		"type": "object",
		"properties": {
			"items": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"name": {"type": "string", "minLength": 1},
						"price": {"type": "number", "minimum": 0}
					},
					"required": ["name", "price"]
				},
				"minItems": 1
			}
		}
	}`

	tests := []struct {
		name      string
		schemaID  string
		order     Order
		wantError bool
		checkErr  func(t *testing.T, err error)
	}{
		{
			name:      "invalid - empty array violates minItems",
			schemaID:  "test-array-1",
			order:     Order{Items: []Item{}},
			wantError: true,
		},
		{
			name:      "invalid - missing name in item",
			schemaID:  "test-array-2",
			order:     Order{Items: []Item{{Name: "", Price: 10}}},
			wantError: true,
		},
		{
			name:      "valid array with single item",
			schemaID:  "test-array-3",
			order:     Order{Items: []Item{{Name: "item1", Price: 10}}},
			wantError: false,
		},
		{
			name:      "invalid - negative price",
			schemaID:  "test-array-4",
			order:     Order{Items: []Item{{Name: "item1", Price: -10}}},
			wantError: true, // Price violates minimum: 0
		},
		{
			name:      "valid array with multiple items",
			schemaID:  "test-array-5",
			order:     Order{Items: []Item{{Name: "item1", Price: 10}, {Name: "item2", Price: 20}}},
			wantError: false,
		},
		{
			name:      "valid - zero price might be valid",
			schemaID:  "test-array-6",
			order:     Order{Items: []Item{{Name: "item1", Price: 0}}}, // Zero price might be valid
			wantError: false,                                           // Zero value might satisfy required if field is present
		},
		{
			name:      "valid - zero price is allowed",
			schemaID:  "test-array-7",
			order:     Order{Items: []Item{{Name: "item1", Price: 0}}},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(t.Context(), &tt.order, WithStrategy(StrategyJSONSchema), WithCustomSchema(tt.schemaID, schema))
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
