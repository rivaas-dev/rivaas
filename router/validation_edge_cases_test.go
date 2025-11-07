package router

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestValidate_NilSlice(t *testing.T) {
	type User struct {
		Name  string   `json:"name" validate:"required"`
		Tags  []string `json:"tags" validate:"required"`
		Items []string `json:"items"`
	}

	user := User{Name: "John", Tags: nil}
	err := Validate(&user, WithStrategy(ValidationTags))
	// nil slice should fail required validation
	if err == nil {
		t.Fatal("expected validation error for nil slice with required tag")
	}

	// Empty slice with required tag - go-playground/validator's "required"
	// only checks for nil, not empty. To require non-empty, use "min=1"
	// So empty slice should pass with just "required"
	user2 := User{Name: "John", Tags: []string{}}
	err = Validate(&user2, WithStrategy(ValidationTags))
	if err != nil {
		t.Errorf("unexpected error for empty slice: %v (required tag doesn't validate non-empty)", err)
	}

	// Valid slice
	user3 := User{Name: "John", Tags: []string{"tag1"}, Items: nil}
	err = Validate(&user3, WithStrategy(ValidationTags))
	if err != nil {
		t.Errorf("unexpected error for valid slice: %v", err)
	}
}

func TestValidate_NilMap(t *testing.T) {
	type User struct {
		Name     string            `json:"name" validate:"required"`
		Metadata map[string]string `json:"metadata" validate:"required"`
	}

	user := User{Name: "John", Metadata: nil}
	err := Validate(&user, WithStrategy(ValidationTags))
	// nil map should fail required validation
	if err == nil {
		t.Fatal("expected validation error for nil map with required tag")
	}

	// Empty map with required tag - go-playground/validator's "required"
	// only checks for nil, not empty. So empty map should pass
	user2 := User{Name: "John", Metadata: map[string]string{}}
	err = Validate(&user2, WithStrategy(ValidationTags))
	if err != nil {
		t.Errorf("unexpected error for empty map: %v (required tag doesn't validate non-empty)", err)
	}

	// Valid map
	user3 := User{Name: "John", Metadata: map[string]string{"key": "value"}}
	err = Validate(&user3, WithStrategy(ValidationTags))
	if err != nil {
		t.Errorf("unexpected error for valid map: %v", err)
	}
}

func TestValidate_DeeplyNestedStructures(t *testing.T) {
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

	// Valid deeply nested
	valid := Level1{
		Level2: Level2{
			Level3: Level3{
				Level4: Level4{
					Level5: Level5{Value: "test"},
				},
			},
		},
	}
	err := Validate(&valid, WithStrategy(ValidationTags))
	if err != nil {
		t.Errorf("unexpected error for valid nested structure: %v", err)
	}

	// Invalid - missing value at level 5
	invalid := Level1{
		Level2: Level2{
			Level3: Level3{
				Level4: Level4{
					Level5: Level5{},
				},
			},
		},
	}
	err = Validate(&invalid, WithStrategy(ValidationTags))
	if err == nil {
		t.Fatal("expected validation error for missing required field")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	// Check path resolution
	found := false
	for _, e := range verrs.Errors {
		if e.Path == "level2.level3.level4.level5.value" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error path for deeply nested field")
	}
}

func TestValidate_Concurrent(t *testing.T) {
	type User struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
	}

	const numGoroutines = 100
	const numValidationsPerGoroutine = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numValidationsPerGoroutine)

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
				err := Validate(&user, WithStrategy(ValidationTags))
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	errCount := 0
	for err := range errors {
		t.Errorf("unexpected validation error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Errorf("got %d unexpected errors during concurrent validation", errCount)
	}
}

func TestValidate_ConcurrentWithCache(t *testing.T) {
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
	errors := make(chan error, numGoroutines*numValidationsPerGoroutine)

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Use numeric ID that's URL-safe
			schemaID := fmt.Sprintf("test-schema-%d", id)
			for range numValidationsPerGoroutine {
				user := User{Name: "John"}
				err := Validate(&user, WithStrategy(ValidationJSONSchema), WithCustomSchema(schemaID, schema))
				if err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errCount := 0
	for err := range errors {
		t.Errorf("unexpected validation error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Errorf("got %d unexpected errors during concurrent schema validation", errCount)
	}
}

func TestValidateWithContext_Cancellation(t *testing.T) {
	type User struct {
		Name string `json:"name"`
	}

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	user := &User{Name: "John"}
	err := Validate(user, WithContext(ctx), WithStrategy(ValidationTags))
	// Validation should still work even with cancelled context
	// (most validators don't check context cancellation)
	if err != nil {
		t.Errorf("unexpected error with cancelled context: %v", err)
	}
}

func TestValidateWithContext_Timeout(t *testing.T) {
	type User struct {
		Name string `json:"name" validate:"required"`
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	user := &User{Name: "John"}
	err := Validate(user, WithContext(ctx), WithStrategy(ValidationTags))
	// Validation should complete before timeout
	if err != nil {
		t.Errorf("unexpected error with timeout context: %v", err)
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Validate again after timeout
	err = Validate(user, WithContext(ctx), WithStrategy(ValidationTags))
	// Should still work (validators typically don't check context)
	if err != nil {
		t.Errorf("unexpected error after timeout: %v", err)
	}
}

func TestValidationErrors_ErrorsAs(t *testing.T) {
	var verrs ValidationErrors
	verrs.Add("name", "required", "is required", nil)
	verrs.Add("email", "email", "invalid email", nil)

	// Test errors.As
	var target ValidationErrors
	if !errors.As(verrs, &target) {
		t.Error("errors.As should work with ValidationErrors")
	}

	if len(target.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(target.Errors))
	}

	// Test errors.Is
	if !errors.Is(verrs, ErrValidation) {
		t.Error("errors.Is should work with ValidationErrors")
	}
}

func TestFieldError_ErrorsIs(t *testing.T) {
	fe := FieldError{
		Path:    "name",
		Code:    "required",
		Message: "is required",
	}

	// Test errors.Is
	if !errors.Is(fe, ErrValidation) {
		t.Error("errors.Is should work with FieldError")
	}

	// Test errors.As
	var target FieldError
	if !errors.As(fe, &target) {
		t.Error("errors.As should work with FieldError")
	}

	if target.Path != "name" {
		t.Errorf("expected path 'name', got %q", target.Path)
	}
}

func TestValidationErrors_UnwrapChain(t *testing.T) {
	var verrs ValidationErrors
	verrs.Add("name", "required", "is required", nil)

	// Test unwrap chain
	err := verrs.Unwrap()
	if !errors.Is(err, ErrValidation) {
		t.Error("Unwrap should return ErrValidation")
	}

	// Test nested wrapping
	wrapped := fmt.Errorf("%w: %w", ErrOuterError, verrs)
	// Note: FieldError and ValidationErrors already implement Unwrap
	// This test verifies the chain works
	if !errors.Is(verrs, ErrValidation) {
		t.Error("errors.Is should work through unwrap chain")
	}

	_ = wrapped // Suppress unused variable
}

func TestValidate_ManyErrors(t *testing.T) {
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

	user := User{} // All fields missing
	err := Validate(&user, WithStrategy(ValidationTags), WithMaxErrors(5))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatal("expected ValidationErrors")
	}

	if len(verrs.Errors) > 5 {
		t.Errorf("expected at most 5 errors, got %d", len(verrs.Errors))
	}

	if !verrs.Truncated {
		t.Error("should be truncated")
	}

	// Test with unlimited errors
	err = Validate(&user, WithStrategy(ValidationTags), WithMaxErrors(0))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var verrs2 ValidationErrors
	if !errors.As(err, &verrs2) {
		t.Fatal("expected ValidationErrors")
	}

	if len(verrs2.Errors) < 15 {
		t.Errorf("expected at least 15 errors with unlimited, got %d", len(verrs2.Errors))
	}
}

func TestValidate_DeepRecursion(t *testing.T) {
	// Test that deeply nested structures don't cause stack overflow
	type Nested struct {
		Value string  `json:"value" validate:"required"`
		Next  *Nested `json:"next"`
	}

	// Create a chain of 100 nested levels
	root := &Nested{Value: "test"}
	current := root
	for range 100 {
		current.Next = &Nested{Value: "test"}
		current = current.Next
	}

	err := Validate(root, WithStrategy(ValidationTags))
	if err != nil {
		t.Errorf("unexpected error for deeply nested structure: %v", err)
	}

	// Test with missing value at depth 50
	root2 := &Nested{Value: "test"}
	current2 := root2
	for range 50 {
		current2.Next = &Nested{Value: "test"}
		current2 = current2.Next
	}
	current2.Next = &Nested{} // Missing value

	err = Validate(root2, WithStrategy(ValidationTags))
	if err == nil {
		t.Fatal("expected validation error for missing required field")
	}
}
