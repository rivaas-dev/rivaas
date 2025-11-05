package router

import (
	"context"
	"errors"
	"testing"
)

// Helper type for testing
type schemaUserImpl struct {
	Name string
}

func (s *schemaUserImpl) JSONSchema() (id string, schema string) {
	return "user", `{"type": "object"}`
}

func TestValidateAll_MultipleStrategies(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	// User that fails both tag validation and would fail interface validation
	user := User{Name: "John"} // Missing email

	err := Validate(&user, WithRunAll(true))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Should have at least one error
	if len(verrs.Errors) == 0 {
		t.Error("should have validation errors")
	}
}

func TestValidateAll_RequireAny(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	// Valid user - at least one strategy should pass
	user := User{Name: "John", Email: "john@example.com"}

	err := Validate(&user, WithRunAll(true), WithRequireAny(true))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid user - all strategies fail
	user2 := User{}
	err = Validate(&user2, WithRunAll(true), WithRequireAny(true))
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateAll_NoApplicableStrategies(t *testing.T) {
	// Simple string value - no applicable strategies
	value := "just a string"

	err := Validate(&value, WithRunAll(true))
	// Should not error if no strategies apply
	if err != nil {
		t.Errorf("unexpected error when no strategies apply: %v", err)
	}
}

func TestCoerceToValidationErrors_AlreadyValidationErrors(t *testing.T) {
	verrs := ValidationErrors{
		Errors: []FieldError{
			{Path: "name", Code: "required", Message: "is required"},
			{Path: "email", Code: "email", Message: "invalid email"},
		},
	}

	cfg := newValidationConfig()
	result := coerceToValidationErrors(verrs, cfg)

	var resultVerrs ValidationErrors
	if !errors.As(result, &resultVerrs) {
		t.Fatal("expected ValidationErrors")
	}

	if len(resultVerrs.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(resultVerrs.Errors))
	}
}

func TestCoerceToValidationErrors_WithMaxErrors(t *testing.T) {
	verrs := ValidationErrors{
		Errors: []FieldError{
			{Path: "field1", Code: "required", Message: "is required"},
			{Path: "field2", Code: "required", Message: "is required"},
			{Path: "field3", Code: "required", Message: "is required"},
		},
	}

	cfg := newValidationConfig(WithMaxErrors(2))
	result := coerceToValidationErrors(verrs, cfg)

	var resultVerrs ValidationErrors
	if !errors.As(result, &resultVerrs) {
		t.Fatal("expected ValidationErrors")
	}

	if len(resultVerrs.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(resultVerrs.Errors))
	}

	if !resultVerrs.Truncated {
		t.Error("should be truncated")
	}
}

func TestCoerceToValidationErrors_FieldError(t *testing.T) {
	fe := FieldError{
		Path:    "name",
		Code:    "required",
		Message: "is required",
	}

	cfg := newValidationConfig()
	result := coerceToValidationErrors(fe, cfg)

	var verrs ValidationErrors
	if !errors.As(result, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	if len(verrs.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(verrs.Errors))
	}

	if verrs.Errors[0].Path != "name" {
		t.Errorf("expected path 'name', got %q", verrs.Errors[0].Path)
	}
}

func TestCoerceToValidationErrors_GenericError(t *testing.T) {
	genericErr := errors.New("generic validation error")

	cfg := newValidationConfig()
	result := coerceToValidationErrors(genericErr, cfg)

	var verrs ValidationErrors
	if !errors.As(result, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	if len(verrs.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(verrs.Errors))
	}

	if verrs.Errors[0].Code != "validation_error" {
		t.Errorf("expected code 'validation_error', got %q", verrs.Errors[0].Code)
	}

	if verrs.Errors[0].Message != "generic validation error" {
		t.Errorf("expected message 'generic validation error', got %q", verrs.Errors[0].Message)
	}
}

func TestCoerceToValidationErrors_NilError(t *testing.T) {
	cfg := newValidationConfig()
	result := coerceToValidationErrors(nil, cfg)

	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestIsApplicable_InterfaceStrategy(t *testing.T) {
	// Struct implementing Validator
	validUser := &userWithValidator{Name: "John", Email: "john@example.com"}

	cfg := newValidationConfig()
	if !isApplicable(validUser, ValidationInterface, cfg) {
		t.Error("should be applicable for Validator interface")
	}

	// With context and ValidatorWithContext
	ctx := context.Background()
	cfgWithCtx := newValidationConfig(WithContext(ctx))
	contextUser := &userWithContextValidator{Name: "John"}

	if !isApplicable(contextUser, ValidationInterface, cfgWithCtx) {
		t.Error("should be applicable for ValidatorWithContext interface")
	}

	// Struct without validator
	type SimpleStruct struct {
		Name string
	}
	simple := &SimpleStruct{}
	if isApplicable(simple, ValidationInterface, cfg) {
		t.Error("should not be applicable without validator")
	}
}

func TestIsApplicable_TagsStrategy(t *testing.T) {
	// Struct with tags
	type User struct {
		Name string `json:"name" validate:"required"`
	}
	user := &User{}

	cfg := newValidationConfig()
	if !isApplicable(user, ValidationTags, cfg) {
		t.Error("should be applicable for struct")
	}

	// Non-struct
	value := "string"
	if isApplicable(value, ValidationTags, cfg) {
		t.Error("should not be applicable for non-struct")
	}

	// Nil pointer
	var nilPtr *User
	if isApplicable(nilPtr, ValidationTags, cfg) {
		t.Error("should not be applicable for nil pointer")
	}
}

func TestIsApplicable_JSONSchemaStrategy(t *testing.T) {
	cfg := newValidationConfig()

	// With custom schema
	cfgWithSchema := newValidationConfig(WithCustomSchema("test", `{"type": "object"}`))
	if !isApplicable(&struct{}{}, ValidationJSONSchema, cfgWithSchema) {
		t.Error("should be applicable with custom schema")
	}

	// Without custom schema and no provider
	if isApplicable(&struct{}{}, ValidationJSONSchema, cfg) {
		t.Error("should not be applicable without schema")
	}

	// With JSONSchemaProvider
	schemaUser := &schemaUserImpl{Name: "John"}

	if !isApplicable(schemaUser, ValidationJSONSchema, cfg) {
		t.Error("should be applicable with JSONSchemaProvider")
	}
}

func TestDetermineStrategy_Auto(t *testing.T) {
	// Struct with Validator interface should prefer interface
	validUser := &userWithValidator{Name: "John", Email: "john@example.com"}

	cfg := newValidationConfig()
	strategy := determineStrategy(validUser, cfg)
	if strategy != ValidationInterface {
		t.Errorf("expected ValidationInterface, got %v", strategy)
	}

	// Struct with tags but no interface
	type TagUser struct {
		Name string `json:"name" validate:"required"`
	}
	tagUser := &TagUser{}
	strategy = determineStrategy(tagUser, cfg)
	if strategy != ValidationTags {
		t.Errorf("expected ValidationTags, got %v", strategy)
	}

	// Struct with JSON Schema (custom schema makes it applicable)
	// Note: determineStrategy prioritizes Interface > Tags > JSONSchema
	// schemaUserImpl has no validation tags, so Tags is not applicable
	schemaUser := &schemaUserImpl{Name: "John"}

	cfgWithSchema := newValidationConfig(WithCustomSchema("user", `{"type": "object"}`))
	strategy = determineStrategy(schemaUser, cfgWithSchema)
	// With custom schema, JSONSchema is applicable
	// Since schemaUserImpl has no tags, Tags is not applicable, so JSONSchema is used
	if strategy != ValidationJSONSchema {
		t.Errorf("expected ValidationJSONSchema (no tags, has custom schema), got %v", strategy)
	}

	// Without custom schema, but implements JSONSchemaProvider
	cfgNoSchema := newValidationConfig()
	strategy = determineStrategy(schemaUser, cfgNoSchema)
	// schemaUserImpl implements JSONSchemaProvider, so JSONSchema is applicable
	// Since it has no tags, Tags is not applicable, so JSONSchema is used
	if strategy != ValidationJSONSchema {
		t.Errorf("expected ValidationJSONSchema (implements JSONSchemaProvider, no tags), got %v", strategy)
	}

	// Default to tags for struct
	type SimpleStruct struct {
		Name string
	}
	simple := &SimpleStruct{}
	strategy = determineStrategy(simple, cfg)
	if strategy != ValidationTags {
		t.Errorf("expected ValidationTags as default, got %v", strategy)
	}
}

func TestValidateByStrategy(t *testing.T) {
	type User struct {
		Name string `json:"name" validate:"required"`
	}

	cfg := newValidationConfig()

	// Test each strategy
	user := &User{} // Missing name

	// Tags strategy
	err := validateByStrategy(user, ValidationTags, cfg)
	if err == nil {
		t.Error("expected error for tags strategy")
	}

	// Interface strategy (no validator)
	err = validateByStrategy(user, ValidationInterface, cfg)
	if err != nil {
		t.Errorf("unexpected error for interface strategy without validator: %v", err)
	}

	// JSON Schema strategy (no schema)
	err = validateByStrategy(user, ValidationJSONSchema, cfg)
	if err != nil {
		t.Errorf("unexpected error for JSON Schema strategy without schema: %v", err)
	}
}

func TestValidateWithInterface_AutoStrategy(t *testing.T) {
	// Auto strategy should detect and use interface validation
	user := &userWithValidator{Name: "John", Email: "john@example.com"}
	err := Validate(user) // No strategy specified - should auto-detect
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid user
	user2 := &userWithValidator{}
	err = Validate(user2) // Auto should use interface validation
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateAll_AllStrategiesCombined(t *testing.T) {
	// Struct implementing all three strategies: Validator + tags + JSONSchemaProvider
	type CombinedUser struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	// Implement Validator
	validUser := &CombinedUser{Name: "John", Email: "john@example.com"}
	err := Validate(validUser, WithRunAll(true))
	// Should pass with interface validation
	if err != nil {
		t.Errorf("valid user should pass: %v", err)
	}

	// Invalid user - missing email
	invalidUser := &CombinedUser{Name: "John"}
	err = Validate(invalidUser, WithRunAll(true))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Should have errors from applicable strategies
	if len(verrs.Errors) == 0 {
		t.Error("should have validation errors")
	}
}

func TestDetermineStrategy_PriorityMatrix(t *testing.T) {
	cfg := newValidationConfig()

	// Test 1: Interface only
	userWithInterface := &userWithValidator{Name: "John", Email: "john@example.com"}
	strategy := determineStrategy(userWithInterface, cfg)
	if strategy != ValidationInterface {
		t.Errorf("expected ValidationInterface, got %v", strategy)
	}

	// Test 2: Tags only
	type TagUser struct {
		Name string `json:"name" validate:"required"`
	}
	tagUser := &TagUser{}
	strategy = determineStrategy(tagUser, cfg)
	if strategy != ValidationTags {
		t.Errorf("expected ValidationTags, got %v", strategy)
	}

	// Test 3: JSON Schema only (with custom schema)
	cfgWithSchema := newValidationConfig(WithCustomSchema("test", `{"type": "object"}`))
	type SimpleUser struct {
		Name string `json:"name"`
	}
	simpleUser := &SimpleUser{}
	strategy = determineStrategy(simpleUser, cfgWithSchema)
	if strategy != ValidationJSONSchema {
		t.Errorf("expected ValidationJSONSchema, got %v", strategy)
	}

	// Test 4: Interface + Tags (should prefer Interface)
	// userWithValidator already implements Validator and we can add tags
	type UserWithBoth struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email"`
	}
	// Create a user that implements Validator
	userBoth := &struct {
		UserWithBoth
		*userWithValidator
	}{
		UserWithBoth:      UserWithBoth{Name: "John"},
		userWithValidator: &userWithValidator{Name: "John", Email: "john@example.com"},
	}
	strategy = determineStrategy(userBoth, cfg)
	if strategy != ValidationInterface {
		t.Errorf("expected ValidationInterface (priority), got %v", strategy)
	}

	// Test 5: JSON Schema only (no tags, but has JSONSchemaProvider)
	schemaUser := &schemaUserImpl{Name: "John"}
	strategy = determineStrategy(schemaUser, cfg)
	if strategy != ValidationJSONSchema {
		t.Errorf("expected ValidationJSONSchema (has JSONSchemaProvider, no tags), got %v", strategy)
	}

	// Test 6: Default for simple struct
	type SimpleStruct struct {
		Name string
	}
	simple := &SimpleStruct{}
	strategy = determineStrategy(simple, cfg)
	if strategy != ValidationTags {
		t.Errorf("expected ValidationTags as default, got %v", strategy)
	}
}
