package router

import (
	"context"
	"errors"
	"testing"
)

func TestValidate_NilValue(t *testing.T) {
	err := Validate(nil)
	if err == nil {
		t.Fatal("expected error for nil value")
	}
}

func TestValidate_NilPointer(t *testing.T) {
	var ptr *struct {
		Name string `json:"name" validate:"required"`
	}
	err := Validate(ptr)
	if err == nil {
		t.Fatal("expected error for nil pointer")
	}
	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}
	if verrs.Errors[0].Code != "nil_pointer" {
		t.Errorf("expected code 'nil_pointer', got %q", verrs.Errors[0].Code)
	}
}

func TestValidationErrors_HasErrors(t *testing.T) {
	var verrs ValidationErrors
	if verrs.HasErrors() {
		t.Error("empty errors should not have errors")
	}

	verrs.Add("name", "required", "is required", nil)
	if !verrs.HasErrors() {
		t.Error("should have errors")
	}
}

func TestValidationErrors_Is(t *testing.T) {
	var verrs ValidationErrors
	verrs.Add("name", "required", "is required", nil)
	verrs.Add("email", "email", "must be email", nil)

	if !verrs.HasCode("required") {
		t.Error("should find 'required' code")
	}
	if !verrs.HasCode("email") {
		t.Error("should find 'email' code")
	}
	if verrs.HasCode("nonexistent") {
		t.Error("should not find nonexistent code")
	}
}

func TestValidationErrors_Sort(t *testing.T) {
	var verrs ValidationErrors
	verrs.Add("z", "code1", "msg1", nil)
	verrs.Add("a", "code2", "msg2", nil)
	verrs.Add("a", "code1", "msg3", nil)

	verrs.Sort()

	if verrs.Errors[0].Path != "a" {
		t.Errorf("expected first path 'a', got %q", verrs.Errors[0].Path)
	}
	if verrs.Errors[0].Code != "code1" {
		t.Errorf("expected first code 'code1', got %q", verrs.Errors[0].Code)
	}
	if verrs.Errors[1].Path != "a" {
		t.Errorf("expected second path 'a', got %q", verrs.Errors[1].Path)
	}
	if verrs.Errors[1].Code != "code2" {
		t.Errorf("expected second code 'code2', got %q", verrs.Errors[1].Code)
	}
	if verrs.Errors[2].Path != "z" {
		t.Errorf("expected third path 'z', got %q", verrs.Errors[2].Path)
	}
}

func TestValidationErrors_Unwrap(t *testing.T) {
	var verrs ValidationErrors
	verrs.Add("name", "required", "is required", nil)

	err := verrs.Unwrap()
	if !errors.Is(err, ErrValidation) {
		t.Error("Unwrap should return ErrValidation")
	}
}

func TestFieldError_Error(t *testing.T) {
	fe := FieldError{
		Path:    "name",
		Code:    "required",
		Message: "is required",
	}

	msg := fe.Error()
	expected := "name: is required"
	if msg != expected {
		t.Errorf("expected %q, got %q", expected, msg)
	}

	// FieldError without path
	fe2 := FieldError{
		Code:    "required",
		Message: "is required",
	}
	msg2 := fe2.Error()
	if msg2 != "is required" {
		t.Errorf("expected 'is required', got %q", msg2)
	}
}

func TestFieldError_Unwrap(t *testing.T) {
	fe := FieldError{
		Path:    "name",
		Code:    "required",
		Message: "is required",
	}

	err := fe.Unwrap()
	if !errors.Is(err, ErrValidation) {
		t.Error("Unwrap should return ErrValidation")
	}
}

func TestValidationErrors_AddError(t *testing.T) {
	var verrs ValidationErrors

	// Add FieldError
	fe := FieldError{
		Path:    "name",
		Code:    "required",
		Message: "is required",
	}
	verrs.AddError(fe)
	if len(verrs.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(verrs.Errors))
	}

	// Add ValidationErrors
	var verrs2 ValidationErrors
	verrs2.Add("email", "email", "invalid email", nil)
	verrs.AddError(verrs2)
	if len(verrs.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(verrs.Errors))
	}

	// Add generic error
	verrs.AddError(errors.New("generic error"))
	if len(verrs.Errors) != 3 {
		t.Errorf("expected 3 errors, got %d", len(verrs.Errors))
	}
	if verrs.Errors[2].Code != "validation_error" {
		t.Errorf("expected code 'validation_error', got %q", verrs.Errors[2].Code)
	}

	// Add nil error (should be ignored)
	initialCount := len(verrs.Errors)
	verrs.AddError(nil)
	if len(verrs.Errors) != initialCount {
		t.Error("nil error should be ignored")
	}
}

func TestValidationErrors_Error(t *testing.T) {
	var verrs ValidationErrors
	if verrs.Error() != "" {
		t.Error("empty errors should return empty string")
	}

	verrs.Add("name", "required", "is required", nil)
	msg := verrs.Error()
	if msg == "" {
		t.Error("should have error message")
	}
	if !contains(msg, "name") {
		t.Error("error message should contain field name")
	}

	// Multiple errors
	verrs.Add("email", "email", "invalid email", nil)
	msg = verrs.Error()
	if !contains(msg, "validation failed") {
		t.Error("multiple errors should have 'validation failed' prefix")
	}

	// With truncation
	verrs.Truncated = true
	msg = verrs.Error()
	if !contains(msg, "truncated") {
		t.Error("truncated errors should indicate truncation")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
func TestPresenceMap_Has(t *testing.T) {
	pm := PresenceMap{
		"name":  true,
		"email": true,
	}

	if !pm.Has("name") {
		t.Error("should have 'name'")
	}
	if !pm.Has("email") {
		t.Error("should have 'email'")
	}
	if pm.Has("nonexistent") {
		t.Error("should not have 'nonexistent'")
	}
}

func TestPresenceMap_HasPrefix(t *testing.T) {
	pm := PresenceMap{
		"address":      true,
		"address.city": true,
		"address.zip":  true,
		"items":        true,
		"items.0":      true,
		"items.0.name": true,
	}

	if !pm.HasPrefix("address") {
		t.Error("should have 'address' prefix")
	}
	if !pm.HasPrefix("address.city") {
		t.Error("should have 'address.city' prefix")
	}
	if !pm.HasPrefix("items") {
		t.Error("should have 'items' prefix")
	}
	if !pm.HasPrefix("items.0") {
		t.Error("should have 'items.0' prefix")
	}
	if pm.HasPrefix("nonexistent") {
		t.Error("should not have 'nonexistent' prefix")
	}
}

func TestPresenceMap_LeafPaths(t *testing.T) {
	pm := PresenceMap{
		"name":         true,
		"address":      true,
		"address.city": true,
		"address.zip":  true,
		"items":        true,
		"items.0":      true,
		"items.0.name": true,
		"items.1":      true,
	}

	leaves := pm.LeafPaths()

	expected := map[string]bool{
		"name":         true,
		"address.city": true,
		"address.zip":  true,
		"items.0.name": true,
		"items.1":      true,
	}

	if len(leaves) != len(expected) {
		t.Errorf("expected %d leaves, got %d", len(expected), len(leaves))
	}

	for _, leaf := range leaves {
		if !expected[leaf] {
			t.Errorf("unexpected leaf: %q", leaf)
		}
	}
}

func TestValidatorInterface_WithContext(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
	}

	// Test struct without Validate method should pass
	impl := &TestStruct{Name: "test"}
	ctx := context.Background()
	err := Validate(impl, WithContext(ctx))
	// Interface validation should pass (no Validate method)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidationStrategy_String(t *testing.T) {
	strategies := []ValidationStrategy{
		ValidationAuto,
		ValidationTags,
		ValidationJSONSchema,
		ValidationInterface,
	}

	for _, s := range strategies {
		if s < 0 || s > ValidationInterface {
			t.Errorf("invalid strategy: %d", s)
		}
	}
}
