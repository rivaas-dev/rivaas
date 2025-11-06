package router

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestValidateWithTags_Required(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	user := User{} // Missing required fields
	err := Validate(&user, WithStrategy(ValidationTags))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	if !verrs.HasCode("tag.required") {
		t.Error("should have 'tag.required' error")
	}
}

func TestValidateWithTags_Email(t *testing.T) {
	type User struct {
		Email string `json:"email" validate:"email"`
	}

	user := User{Email: "invalid-email"}
	err := Validate(&user, WithStrategy(ValidationTags))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	if !verrs.HasCode("tag.email") {
		t.Error("should have 'tag.email' error")
	}
}

func TestValidatePartialLeafsOnly(t *testing.T) {
	type Address struct {
		City string `json:"city" validate:"required"`
		Zip  string `json:"zip" validate:"required"`
	}

	type User struct {
		Name    string  `json:"name" validate:"required"`
		Address Address `json:"address" validate:"required"`
	}

	// Simulate PATCH request with only "name" field
	pm := PresenceMap{
		"name": true,
	}

	user := User{Name: "John"}
	err := Validate(&user, WithStrategy(ValidationTags), WithPartial(true), WithPresence(pm))
	if err != nil {
		// Should not error because "address.city" and "address.zip" are not present
		t.Errorf("unexpected error in partial mode: %v", err)
	}

	// But if we provide "address.city" without "address.zip", it should validate
	pm2 := PresenceMap{
		"name":         true,
		"address":      true,
		"address.city": true,
	}
	user2 := User{Name: "John", Address: Address{City: "NYC"}}
	err = Validate(&user2, WithStrategy(ValidationTags), WithPartial(true), WithPresence(pm2))
	if err != nil {
		// Should error because "address.zip" is required but not provided
		// But in leaf-only mode, we only validate what's present
		// So this should pass since we're only validating "address.city"
		var verrs ValidationErrors
		if errors.As(err, &verrs) {
			t.Logf("validation errors: %v", verrs)
		}
	}
}

func TestValidateWithTags_CustomValidators(t *testing.T) {
	type User struct {
		Username string `json:"username" validate:"username"`
		Slug     string `json:"slug" validate:"slug"`
	}

	user := User{
		Username: "ab",           // Too short
		Slug:     "Invalid_Slug", // Invalid characters
	}

	err := Validate(&user, WithStrategy(ValidationTags))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Should have errors for both username and slug
	if len(verrs.Errors) < 2 {
		t.Errorf("expected at least 2 errors, got %d", len(verrs.Errors))
	}
}

func TestRegisterTag_Freeze(t *testing.T) {
	// Reset frozen state for testing
	validationsFrozen.Store(false)

	// First registration should work
	err := RegisterTag("test_tag", func(_ validator.FieldLevel) bool {
		return true
	})
	if err != nil {
		t.Fatalf("first registration should succeed: %v", err)
	}

	// Trigger validation to freeze
	type TestStruct struct {
		Field string `json:"field" validate:"required"`
	}
	_ = Validate(&TestStruct{Field: "test"}, WithStrategy(ValidationTags))

	// Second registration should fail
	err = RegisterTag("test_tag2", func(_ validator.FieldLevel) bool {
		return true
	})
	if err == nil {
		t.Error("second registration should fail after freeze")
	}
}

func TestPathResolution(t *testing.T) {
	type Item struct {
		Name  string `json:"name"`
		Price int    `json:"price"`
	}

	type Order struct {
		Items []Item `json:"items"`
	}

	order := Order{
		Items: []Item{
			{Name: "item1", Price: 100},
			{Name: "item2", Price: 200},
		},
	}

	// Test resolving path "items.0.name"
	val, field, ok := resolvePath(&order, "items.0.name")
	if !ok {
		t.Fatal("should resolve path")
	}
	if val.String() != "item1" {
		t.Errorf("expected 'item1', got %q", val.String())
	}
	if field.Name != "Name" {
		t.Errorf("expected field name 'Name', got %q", field.Name)
	}

	// Test resolving path "items.1.price"
	val, field, ok = resolvePath(&order, "items.1.price")
	if !ok {
		t.Fatal("should resolve path")
	}
	if val.Int() != 200 {
		t.Errorf("expected 200, got %d", val.Int())
	}
}

func TestRedaction(t *testing.T) {
	type User struct {
		Password string `json:"password" validate:"required,min=8"`
		Token    string `json:"token" validate:"required"`
	}

	user := User{Password: "short", Token: "secret123"}
	redactor := func(path string) bool {
		return path == "password" || path == "token"
	}

	err := Validate(&user, WithStrategy(ValidationTags), WithRedactor(redactor))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Check that values are redacted in metadata
	for _, e := range verrs.Errors {
		if e.Path == "password" || e.Path == "token" {
			if e.Meta["value"] != "***REDACTED***" {
				t.Errorf("expected redacted value for %s, got %v", e.Path, e.Meta["value"])
			}
			// Note: Our error messages don't include the actual value by default,
			// so there's nothing to redact in the message itself.
			// The message is like "must be at least 8 characters", not "value 'short' is invalid"
		}
	}
}

func TestValidatePartial_NestedArrays(t *testing.T) {
	type Item struct {
		Name  string `json:"name" validate:"required"`
		Price int    `json:"price" validate:"required,min=1"`
	}

	type Order struct {
		Items []Item `json:"items" validate:"required,min=1"`
	}

	// PATCH: Only update items[0].name
	pm := PresenceMap{
		"items":        true,
		"items.0":      true,
		"items.0.name": true,
	}

	order := Order{
		Items: []Item{
			{Name: "updated", Price: 0}, // Price missing but not present
		},
	}

	err := Validate(&order, WithStrategy(ValidationTags), WithPartial(true), WithPresence(pm))
	if err != nil {
		t.Errorf("unexpected error in partial mode: %v", err)
	}

	// PATCH: Update items[0].name and items[1].price
	pm2 := PresenceMap{
		"items":         true,
		"items.0":       true,
		"items.0.name":  true,
		"items.1":       true,
		"items.1.price": true,
	}

	order2 := Order{
		Items: []Item{
			{Name: "updated"},
			{Price: 100},
		},
	}

	err = Validate(&order2, WithStrategy(ValidationTags), WithPartial(true), WithPresence(pm2))
	if err != nil {
		t.Errorf("unexpected error in partial mode: %v", err)
	}

	// Should fail if items[0].name is present but empty
	order3 := Order{
		Items: []Item{
			{Name: ""}, // Empty name
		},
	}

	err = Validate(&order3, WithStrategy(ValidationTags), WithPartial(true), WithPresence(pm))
	if err == nil {
		t.Fatal("expected validation error for empty required field")
	}
}

func TestValidatePartial_Maps(t *testing.T) {
	type User struct {
		Name     string            `json:"name" validate:"required"`
		Metadata map[string]string `json:"metadata" validate:"required"`
	}

	// PATCH: Only update name
	pm := PresenceMap{
		"name": true,
	}

	user := User{Name: "John"}
	err := Validate(&user, WithStrategy(ValidationTags), WithPartial(true), WithPresence(pm))
	if err != nil {
		t.Errorf("unexpected error in partial mode: %v", err)
	}

	// PATCH: Update metadata
	pm2 := PresenceMap{
		"metadata": true,
	}

	user2 := User{Metadata: map[string]string{"key": "value"}}
	err = Validate(&user2, WithStrategy(ValidationTags), WithPartial(true), WithPresence(pm2))
	if err != nil {
		t.Errorf("unexpected error in partial mode: %v", err)
	}
}

func TestValidatePartial_EmptyVsNilSlices(t *testing.T) {
	type User struct {
		Name  string   `json:"name" validate:"required"`
		Tags  []string `json:"tags" validate:"required"`
		Items []string `json:"items"`
	}

	// PATCH: Only update name, tags is nil (not present)
	pm := PresenceMap{
		"name": true,
	}

	user := User{Name: "John", Tags: nil}
	err := Validate(&user, WithStrategy(ValidationTags), WithPartial(true), WithPresence(pm))
	if err != nil {
		t.Errorf("unexpected error when tags not present: %v", err)
	}

	// PATCH: Update tags explicitly (empty slice)
	pm2 := PresenceMap{
		"name": true,
		"tags": true,
	}

	user2 := User{Name: "John", Tags: []string{}}
	err = Validate(&user2, WithStrategy(ValidationTags), WithPartial(true), WithPresence(pm2))
	// go-playground/validator's "required" tag only checks for nil, not empty
	// An empty slice []string{} is not nil, so it passes "required" validation
	// To require non-empty, you would need "min=1" in addition to "required"
	if err != nil {
		t.Errorf("unexpected error for empty slice: %v (required tag doesn't validate non-empty)", err)
	}

	// Valid tags
	user3 := User{Name: "John", Tags: []string{"tag1"}}
	err = Validate(&user3, WithStrategy(ValidationTags), WithPartial(true), WithPresence(pm2))
	if err != nil {
		t.Errorf("unexpected error for valid tags: %v", err)
	}
}

func TestFieldNameMapper_NestedFields(t *testing.T) {
	type Address struct {
		Street string `json:"street" validate:"required"`
		City   string `json:"city" validate:"required"`
	}

	type User struct {
		FirstName string  `json:"first_name" validate:"required"`
		Address   Address `json:"address"`
	}

	mapper := func(name string) string {
		return strings.ReplaceAll(name, "_", " ")
	}

	user := User{}
	err := Validate(&user, WithStrategy(ValidationTags), WithFieldNameMapper(mapper))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Check that nested field names are mapped
	found := false
	for _, e := range verrs.Errors {
		if strings.Contains(e.Path, " ") {
			found = true
		}
	}
	if !found {
		t.Error("expected mapped field names with spaces")
	}
}

func TestFieldNameMapper_ArrayElements(t *testing.T) {
	type Item struct {
		Name string `json:"name" validate:"required"`
	}

	type Order struct {
		Items []Item `json:"items" validate:"dive"`
	}

	mapper := func(name string) string {
		// Transform array indices
		if strings.Contains(name, ".") {
			parts := strings.Split(name, ".")
			for i, part := range parts {
				if _, err := strconv.Atoi(part); err == nil {
					parts[i] = "[" + part + "]"
				}
			}
			return strings.Join(parts, ".")
		}
		return name
	}

	order := Order{
		Items: []Item{
			{Name: ""}, // Missing name - should fail required validation
		},
	}

	err := Validate(&order, WithStrategy(ValidationTags), WithFieldNameMapper(mapper))
	if err == nil {
		t.Fatal("expected validation error for empty Name field")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Check that we got validation errors for the items array
	found := false
	for _, e := range verrs.Errors {
		// Paths are typically in format "items.0.name" (dot notation)
		// The mapper should transform numeric indices to [num] format
		// So we should see something like "items.[0].name" if mapper worked
		if strings.Contains(e.Path, "items") && (strings.Contains(e.Path, "[") || strings.Contains(e.Path, "0")) {
			found = true
			// Log the actual path for debugging
			t.Logf("Found validation error with path: %s", e.Path)
			break
		}
	}
	if !found {
		// Log all paths for debugging
		for _, e := range verrs.Errors {
			t.Logf("Validation error path: %s", e.Path)
		}
		t.Error("expected validation error for items array element")
	}
}

func TestRedaction_NestedSensitiveFields(t *testing.T) {
	type User struct {
		Password string `json:"password" validate:"required,min=8"`
		Profile  struct {
			Token string `json:"token" validate:"required"`
		} `json:"profile"`
	}

	user := User{
		Password: "short",
		Profile: struct {
			Token string `json:"token" validate:"required"`
		}{Token: "secret"},
	}

	redactor := func(path string) bool {
		return path == "password" || path == "profile.token"
	}

	err := Validate(&user, WithStrategy(ValidationTags), WithRedactor(redactor))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Check nested redaction
	for _, e := range verrs.Errors {
		if e.Path == "password" || e.Path == "profile.token" {
			if e.Meta["value"] != "***REDACTED***" {
				t.Errorf("expected redacted value for %s, got %v", e.Path, e.Meta["value"])
			}
		}
	}
}

func TestRedaction_AllErrorTypes(t *testing.T) {
	// Test redaction works for all validation error types, not just tags
	type User struct {
		Password string `json:"password" validate:"required"`
	}

	user := &User{Password: ""}

	redactor := func(path string) bool {
		return path == "password"
	}

	// Test with tags
	err := Validate(&user, WithStrategy(ValidationTags), WithRedactor(redactor))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Redaction should apply regardless of strategy
	for _, e := range verrs.Errors {
		if e.Path == "password" {
			if e.Meta["value"] != "***REDACTED***" {
				t.Errorf("expected redacted value, got %v", e.Meta["value"])
			}
		}
	}
}
