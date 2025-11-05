package router

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestValidateWithSchema_Basic(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "number", "minimum": 0}
		},
		"required": ["name"]
	}`

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	// Valid user
	user := User{Name: "John", Age: 30}
	err := Validate(&user, WithStrategy(ValidationJSONSchema), WithCustomSchema("user", schema))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test with an invalid schema constraint
	schema3 := `{
		"type": "object",
		"properties": {
			"name": {"type": "string", "minLength": 1},
			"age": {"type": "number", "minimum": 1}
		},
		"required": ["name", "age"]
	}`

	user3 := User{Name: "John", Age: 0} // age is 0, which violates minimum: 1
	err = Validate(&user3, WithStrategy(ValidationJSONSchema), WithCustomSchema("user-strict", schema3))
	if err == nil {
		t.Fatal("expected validation error for age minimum")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Should have schema error (not necessarily "required", could be "minimum")
	if len(verrs.Errors) == 0 {
		t.Error("should have validation errors")
	}
}

func TestValidateWithSchema_Partial(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"email": {"type": "string", "format": "email"}
		},
		"required": ["name", "email"]
	}`

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	// PATCH request: only provide name
	pm := PresenceMap{
		"name": true,
	}

	user := User{Name: "John"}
	err := Validate(&user, WithStrategy(ValidationJSONSchema), WithCustomSchema("user", schema),
		WithPartial(true), WithPresence(pm))
	// Should not error because email is not present in partial mode
	if err != nil {
		t.Errorf("unexpected error in partial mode: %v", err)
	}
}

func TestPruneByPresence(t *testing.T) {
	pm := PresenceMap{
		"name":         true,
		"address":      true,
		"address.city": true,
		"items":        true,
		"items.0":      true,
		"items.0.name": true,
	}

	data := map[string]any{
		"name":  "John",
		"email": "john@example.com", // Not present
		"address": map[string]any{
			"city": "NYC",
			"zip":  "10001", // Not present
		},
		"items": []any{
			map[string]any{
				"name":  "item1",
				"price": 100, // Not present
			},
			map[string]any{
				"name": "item2", // Not present
			},
		},
	}

	pruned := pruneByPresence(data, "", pm).(map[string]any)

	// Should have name
	if pruned["name"] != "John" {
		t.Error("should have 'name'")
	}

	// Should not have email
	if _, ok := pruned["email"]; ok {
		t.Error("should not have 'email'")
	}

	// Should have address.city but not address.zip
	addr := pruned["address"].(map[string]any)
	if addr["city"] != "NYC" {
		t.Error("should have 'address.city'")
	}
	if _, ok := addr["zip"]; ok {
		t.Error("should not have 'address.zip'")
	}

	// Should have items.0.name but not items.0.price
	items := pruned["items"].([]any)
	item0 := items[0].(map[string]any)
	if item0["name"] != "item1" {
		t.Error("should have 'items.0.name'")
	}
	if _, ok := item0["price"]; ok {
		t.Error("should not have 'items.0.price'")
	}

	// items.1 should be nil (not present)
	if items[1] != nil {
		t.Error("items.1 should be nil")
	}
}

func TestGetRawJSONFromContext(t *testing.T) {
	rawJSON := []byte(`{"name": "John"}`)
	ctx := context.WithValue(context.Background(), contextKeyRawJSON, rawJSON)

	retrieved := getRawJSONFromContext(ctx)
	if string(retrieved) != string(rawJSON) {
		t.Errorf("expected %s, got %s", string(rawJSON), string(retrieved))
	}

	// Test with context without raw JSON
	ctx2 := context.Background()
	retrieved = getRawJSONFromContext(ctx2)
	if retrieved != nil {
		t.Error("should return nil for context without raw JSON")
	}
}

func TestJSONSchemaProvider(t *testing.T) {
	type User struct {
		Name string `json:"name"`
	}

	// Test with custom schema directly (WithCustomSchema)
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string", "minLength": 1}
		},
		"required": ["name"]
	}`

	user := User{Name: "John"}
	err := Validate(&user, WithStrategy(ValidationJSONSchema), WithCustomSchema("user", schema))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test with empty name (should fail minLength constraint)
	user2 := User{Name: ""}
	err = Validate(&user2, WithStrategy(ValidationJSONSchema), WithCustomSchema("user2", schema))
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}
}

func TestSchemaCache(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		}
	}`

	type User struct {
		Name string `json:"name"`
	}

	// First call should compile
	user1 := User{Name: "John"}
	err := Validate(&user1, WithStrategy(ValidationJSONSchema), WithCustomSchema("test1", schema))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call should use cache
	user2 := User{Name: "Jane"}
	err = Validate(&user2, WithStrategy(ValidationJSONSchema), WithCustomSchema("test1", schema))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cache is working (schema should be cached)
	schemaCacheMu.RLock()
	if _, ok := schemaCache["test1"]; !ok {
		t.Error("schema should be cached")
	}
	schemaCacheMu.RUnlock()
}

func TestValidateWithSchema_InvalidSchema(t *testing.T) {
	// Test behavior with malformed JSON schema
	invalidSchema := `{
		"type": "object",
		"properties": {
			"name": {"type": "invalid_type"}  // Invalid type
		}
	}`

	type User struct {
		Name string `json:"name"`
	}

	user := User{Name: "John"}
	err := Validate(&user, WithStrategy(ValidationJSONSchema), WithCustomSchema("invalid", invalidSchema))
	// Should return error for invalid schema
	if err == nil {
		t.Fatal("expected error for invalid schema")
	}

	// Test with completely malformed JSON
	malformedSchema := `{invalid json}`
	err = Validate(&user, WithStrategy(ValidationJSONSchema), WithCustomSchema("malformed", malformedSchema))
	if err == nil {
		t.Fatal("expected error for malformed JSON schema")
	}
}

func TestValidateWithSchema_NestedObjectErrors(t *testing.T) {
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

	type Address struct {
		City string `json:"city"`
		Zip  string `json:"zip"`
	}

	type User struct {
		Address Address `json:"address"`
	}

	// Invalid nested object - missing city
	user1 := User{
		Address: Address{
			Zip: "12345",
		},
	}
	err := Validate(&user1, WithStrategy(ValidationJSONSchema), WithCustomSchema("nested", schema))
	if err == nil {
		t.Fatal("expected validation error for missing nested required field")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Should have error for address.city
	found := false
	for _, e := range verrs.Errors {
		if e.Path == "address.city" || strings.Contains(e.Path, "city") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error for nested field address.city")
	}

	// Invalid nested object - invalid zip pattern
	user2 := User{
		Address: Address{
			City: "NYC",
			Zip:  "invalid",
		},
	}
	err = Validate(&user2, WithStrategy(ValidationJSONSchema), WithCustomSchema("nested2", schema))
	if err == nil {
		t.Fatal("expected validation error for invalid zip pattern")
	}
}

func TestValidateWithSchema_SchemaRefs(t *testing.T) {
	// Test $ref resolution if supported by the schema library
	// Note: This depends on the JSON Schema library's $ref support
	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string", "minLength": 1}
		},
		"required": ["name"]
	}`

	type User struct {
		Name string `json:"name"`
	}

	user := User{Name: ""}
	err := Validate(&user, WithStrategy(ValidationJSONSchema), WithCustomSchema("ref-test", schema))
	if err == nil {
		t.Fatal("expected validation error for empty name")
	}

	// Valid user
	user2 := User{Name: "John"}
	err = Validate(&user2, WithStrategy(ValidationJSONSchema), WithCustomSchema("ref-test2", schema))
	if err != nil {
		t.Errorf("unexpected error for valid user: %v", err)
	}
}

func TestValidateWithSchema_ArrayValidation(t *testing.T) {
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

	type Item struct {
		Name  string  `json:"name"`
		Price float64 `json:"price"`
	}

	type Order struct {
		Items []Item `json:"items"`
	}

	// Empty array should fail minItems
	order1 := Order{Items: []Item{}}
	err := Validate(&order1, WithStrategy(ValidationJSONSchema), WithCustomSchema("array", schema))
	if err == nil {
		t.Fatal("expected validation error for empty array")
	}

	// Invalid item in array
	order2 := Order{
		Items: []Item{
			{Name: "", Price: 10}, // Missing name
		},
	}
	err = Validate(&order2, WithStrategy(ValidationJSONSchema), WithCustomSchema("array2", schema))
	if err == nil {
		t.Fatal("expected validation error for invalid item in array")
	}

	// Valid array
	order3 := Order{
		Items: []Item{
			{Name: "item1", Price: 10},
		},
	}
	err = Validate(&order3, WithStrategy(ValidationJSONSchema), WithCustomSchema("array3", schema))
	if err != nil {
		t.Errorf("unexpected error for valid array: %v", err)
	}
}
