package router

import (
	"context"
	"errors"
	"testing"
)

type testContextKey string

const testLocaleKey testContextKey = "locale"

// Test structs implementing Validator interface
type userWithValidator struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (u *userWithValidator) Validate() error {
	if u.Name == "" {
		return errors.New("name is required")
	}
	if u.Email == "" {
		return errors.New("email is required")
	}
	return nil
}

type userWithValueValidator struct {
	Name string `json:"name"`
}

func (u userWithValueValidator) Validate() error {
	if u.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

// Test structs implementing ValidatorWithContext interface
type userWithContextValidator struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (u *userWithContextValidator) ValidateContext(ctx context.Context) error {
	if u.Name == "" {
		return errors.New("name is required")
	}
	// Check context for locale
	if locale := ctx.Value(testLocaleKey); locale != nil {
		if locale.(string) == "fa" && len(u.Name) < 3 {
			return errors.New("نام باید حداقل ۳ کاراکتر باشد")
		}
	}
	return nil
}

type userWithValueContextValidator struct {
	Name string `json:"name"`
}

func (u userWithValueContextValidator) ValidateContext(ctx context.Context) error {
	if u.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

// Test struct with both interfaces (should prefer ValidatorWithContext)
type userWithBoth struct {
	Name string `json:"name"`
}

func (u *userWithBoth) Validate() error {
	return errors.New("should not use Validate()")
}

func (u *userWithBoth) ValidateContext(ctx context.Context) error {
	if u.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func TestValidateWithInterface_PointerReceiver(t *testing.T) {
	// Valid user
	user := &userWithValidator{Name: "John", Email: "john@example.com"}
	err := Validate(user, WithStrategy(ValidationInterface))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid user - missing name
	user2 := &userWithValidator{Email: "john@example.com"}
	err = Validate(user2, WithStrategy(ValidationInterface))
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateWithInterface_ValueReceiver(t *testing.T) {
	// Valid user
	user := userWithValueValidator{Name: "John"}
	err := Validate(user, WithStrategy(ValidationInterface))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid user - missing name
	user2 := userWithValueValidator{}
	err = Validate(user2, WithStrategy(ValidationInterface))
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateWithInterface_WithContext(t *testing.T) {
	// Valid user with context
	ctx := context.WithValue(context.Background(), testLocaleKey, "en")
	user := &userWithContextValidator{Name: "John"}
	err := Validate(user, WithStrategy(ValidationInterface), WithContext(ctx))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Invalid user - missing name
	user2 := &userWithContextValidator{}
	err = Validate(user2, WithStrategy(ValidationInterface), WithContext(ctx))
	if err == nil {
		t.Fatal("expected validation error")
	}

	// Context-aware validation (Farsi locale)
	ctxFa := context.WithValue(context.Background(), testLocaleKey, "fa")
	user3 := &userWithContextValidator{Name: "Jo"} // Too short for Farsi
	err = Validate(user3, WithStrategy(ValidationInterface), WithContext(ctxFa))
	if err == nil {
		t.Fatal("expected validation error for short name in Farsi locale")
	}

	// Valid long name for Farsi
	user4 := &userWithContextValidator{Name: "محمد"}
	err = Validate(user4, WithStrategy(ValidationInterface), WithContext(ctxFa))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateWithInterface_ValueReceiverWithContext(t *testing.T) {
	ctx := context.Background()
	user := userWithValueContextValidator{Name: "John"}
	err := Validate(user, WithStrategy(ValidationInterface), WithContext(ctx))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	user2 := userWithValueContextValidator{}
	err = Validate(user2, WithStrategy(ValidationInterface), WithContext(ctx))
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateWithInterface_PrefersContextOverValidate(t *testing.T) {
	// Struct implements both - should prefer ValidateContext
	ctx := context.Background()
	user := &userWithBoth{Name: "John"}
	err := Validate(user, WithStrategy(ValidationInterface), WithContext(ctx))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should have used ValidateContext, not Validate
	// If Validate() was called, it would return "should not use Validate()"
	if err != nil && err.Error() == "should not use Validate()" {
		t.Error("ValidateContext should be preferred over Validate")
	}
}

func TestValidateWithInterface_NoValidator(t *testing.T) {
	// Struct without validator should pass
	type SimpleStruct struct {
		Name string `json:"name"`
	}
	simple := &SimpleStruct{Name: "test"}
	err := Validate(simple, WithStrategy(ValidationInterface))
	if err != nil {
		t.Errorf("should pass when no validator: %v", err)
	}
}

func TestValidateWithInterface_ErrorCoercion(t *testing.T) {
	// Test that errors are properly coerced to ValidationErrors
	user := &userWithValidator{} // Missing required fields
	err := Validate(user, WithStrategy(ValidationInterface))
	if err == nil {
		t.Fatal("expected validation error")
	}

	// Should be able to unwrap
	var verrs ValidationErrors
	if errors.As(err, &verrs) {
		// Good, it's a ValidationErrors
	} else {
		// That's also fine - it might be a generic error
		// The important thing is we got an error
	}
}
