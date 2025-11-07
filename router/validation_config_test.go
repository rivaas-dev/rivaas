package router

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestWithRunAll(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	user := User{Name: "John"} // Missing email

	// Without RunAll - should stop at first strategy
	err := Validate(&user)
	if err == nil {
		t.Fatal("expected validation error")
	}

	// With RunAll - should run all applicable strategies
	err = Validate(&user, WithRunAll(true))
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestWithRequireAny(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	// Valid user - should pass with requireAny
	user := User{Name: "John", Email: "john@example.com"}
	err := Validate(&user, WithRequireAny(true))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWithMaxErrors(t *testing.T) {
	type User struct {
		Field1 string `json:"field1" validate:"required"`
		Field2 string `json:"field2" validate:"required"`
		Field3 string `json:"field3" validate:"required"`
		Field4 string `json:"field4" validate:"required"`
	}

	user := User{}
	err := Validate(&user, WithMaxErrors(2))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	if len(verrs.Errors) > 2 {
		t.Errorf("expected at most 2 errors, got %d", len(verrs.Errors))
	}

	if !verrs.Truncated {
		t.Error("should be truncated")
	}

	// With maxErrors = 0 (unlimited)
	err = Validate(&user, WithMaxErrors(0))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs2 ValidationErrors
	if !errors.As(err, &verrs2) {
		t.Fatal("expected ValidationErrors")
	}

	if len(verrs2.Errors) < 4 {
		t.Errorf("expected at least 4 errors with unlimited, got %d", len(verrs2.Errors))
	}
}

func TestWithDisallowUnknownFields(t *testing.T) {
	// This option is used in BindJSONStrict, test it via context validation
	// The actual validation happens during binding, not in Validate itself
	// So we test the option is set correctly
	cfg := newValidationConfig(WithDisallowUnknownFields(true))
	if !cfg.disallowUnknownFields {
		t.Error("disallowUnknownFields should be true")
	}

	cfg2 := newValidationConfig(WithDisallowUnknownFields(false))
	if cfg2.disallowUnknownFields {
		t.Error("disallowUnknownFields should be false")
	}
}

func TestWithCustomValidator(t *testing.T) {
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
			return ErrCustomNameRequired
		}
		return nil
	}

	user := &User{}
	err := Validate(user, WithCustomValidator(customValidator))
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(err.Error(), "custom: name is required") {
		t.Errorf("expected custom error message, got %q", err.Error())
	}

	// Valid user
	user2 := &User{Name: "John"}
	err = Validate(user2, WithCustomValidator(customValidator))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWithFieldNameMapper(t *testing.T) {
	type User struct {
		FirstName string `json:"first_name" validate:"required"`
		LastName  string `json:"last_name" validate:"required"`
	}

	mapper := func(name string) string {
		return strings.ReplaceAll(name, "_", " ")
	}

	user := User{}
	err := Validate(&user, WithFieldNameMapper(mapper), WithStrategy(ValidationTags))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Check that field names are mapped
	for _, e := range verrs.Errors {
		if e.Path != "" && !strings.Contains(e.Path, " ") {
			t.Errorf("expected mapped field name with space, got %q", e.Path)
		}
	}
}

func TestWithRedactor(t *testing.T) {
	type User struct {
		Password string `json:"password" validate:"required,min=8"`
		Token    string `json:"token" validate:"required"`
	}

	redactor := func(path string) bool {
		return path == "password" || path == "token"
	}

	user := User{Password: "short", Token: "secret123"}
	err := Validate(&user, WithStrategy(ValidationTags), WithRedactor(redactor))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Check that values are redacted
	for _, e := range verrs.Errors {
		if e.Path == "password" || e.Path == "token" {
			if e.Meta["value"] != "***REDACTED***" {
				t.Errorf("expected redacted value for %s, got %v", e.Path, e.Meta["value"])
			}
		}
	}
}

func TestWithContext(t *testing.T) {
	type contextKey string
	key := contextKey("key")
	ctx := context.WithValue(context.Background(), key, "value")
	cfg := newValidationConfig(WithContext(ctx))

	if cfg.ctx == nil {
		t.Error("context should be set")
	}

	if cfg.ctx.Value(key) != "value" {
		t.Error("context values should be preserved")
	}
}

func TestWithPresence(t *testing.T) {
	pm := PresenceMap{
		"name":  true,
		"email": true,
	}

	cfg := newValidationConfig(WithPresence(pm))

	if cfg.presence == nil {
		t.Error("presence map should be set")
	}

	if !cfg.presence.Has("name") {
		t.Error("presence map should contain 'name'")
	}
}

func TestWithCustomSchema(t *testing.T) {
	schema := `{"type": "object", "properties": {"name": {"type": "string"}}}`
	id := "user-schema"

	cfg := newValidationConfig(WithCustomSchema(id, schema))

	if cfg.customSchemaID != id {
		t.Errorf("expected schema ID %q, got %q", id, cfg.customSchemaID)
	}

	if cfg.customSchema != schema {
		t.Errorf("expected schema %q, got %q", schema, cfg.customSchema)
	}
}

func TestNewValidationConfig_Defaults(t *testing.T) {
	cfg := newValidationConfig()

	if cfg.strategy != ValidationAuto {
		t.Errorf("expected ValidationAuto, got %v", cfg.strategy)
	}

	if cfg.runAll {
		t.Error("runAll should default to false")
	}

	if cfg.requireAny {
		t.Error("requireAny should default to false")
	}

	if cfg.partial {
		t.Error("partial should default to false")
	}

	if cfg.maxErrors != 0 {
		t.Errorf("maxErrors should default to 0, got %d", cfg.maxErrors)
	}

	if cfg.disallowUnknownFields {
		t.Error("disallowUnknownFields should default to false")
	}

	if cfg.ctx == nil {
		t.Error("ctx should default to context.Background()")
	}
}
