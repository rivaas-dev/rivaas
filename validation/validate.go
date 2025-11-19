/*
Package validation provides flexible, multi-strategy validation for Go structs.

# Validation Strategies

The package supports three validation strategies:

1. Struct Tags - Using go-playground/validator tags (e.g., `validate:"required,email"`)
2. JSON Schema - RFC-compliant JSON Schema validation via JSONSchemaProvider interface
3. Custom Interfaces - Implement Validator or ValidatorWithContext for custom validation logic

The package automatically selects the best strategy based on the value type, or you can
explicitly choose a strategy using WithStrategy().

# Usage

Basic validation with struct tags:

	type User struct {
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"min=18"`
	}

	user := User{Email: "invalid", Age: 15}
	if err := validation.Validate(ctx, &user); err != nil {
		var verr *validation.Error
		if errors.As(err, &verr) {
			for _, fieldErr := range verr.Fields {
				fmt.Printf("%s: %s\n", fieldErr.Path, fieldErr.Message)
			}
		}
	}

Partial validation for PATCH requests:

	presence := validation.ComputePresence(rawJSON)
	err := validation.ValidatePartial(ctx, &user, presence)

Custom validation with interface:

	type User struct {
		Email string
	}

	func (u *User) Validate() error {
		if !strings.Contains(u.Email, "@") {
			return errors.New("email must contain @")
		}
		return nil
	}

	err := validation.Validate(ctx, &user)

# Performance

The validation package uses several optimizations:
- Path and field map caching to avoid repeated reflection
- Schema compilation caching with LRU eviction
- Pre-compiled regular expressions for built-in validators
- Zero-copy JSON when raw JSON is available in context

# Thread Safety

All validation functions are safe for concurrent use. Caches use sync.Map for
lock-free reads. Custom validator registration must happen before first use
and is then frozen for thread safety.

# Security

The package includes protections against:
- Stack overflow from deeply nested structures (max depth: 100)
- Unbounded memory usage (configurable limits on errors and fields)
- Sensitive data exposure (redaction support via WithRedactor)
*/
package validation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// Validate validates a value using the specified validation strategy.
// This is the main entry point for validation.
//
// Returns nil if validation passes, or an error if validation fails.
// The error can be type-asserted to *Error for structured field errors.
//
// Example:
//
//	var req CreateUserRequest
//	if err := Validate(ctx, &req); err != nil {
//	    var verr *Error
//	    if errors.As(err, &verr) {
//	        // Handle structured validation errors
//	    }
//	}
//
// With options:
//
//	if err := Validate(ctx, &req,
//	    WithStrategy(StrategyTags),
//	    WithPartial(true),
//	    WithPresence(presenceMap),
//	); err != nil {
//	    // Handle validation error
//	}
func Validate(ctx context.Context, v any, opts ...Option) error {
	if v == nil {
		return &Error{Fields: []FieldError{{Code: "nil", Message: ErrCannotValidateNilValue.Error()}}}
	}

	cfg := defaultConfig(opts...)
	// Use the context parameter if it wasn't explicitly set via WithContext() option
	if !cfg.ctxExplicit {
		cfg.ctx = ctx
	}

	// Handle nil pointers and invalid values
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return &Error{Fields: []FieldError{{Code: "invalid", Message: ErrCannotValidateInvalidValue.Error()}}}
	}

	// Check for nil pointers (but preserve pointer for interface validation)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return &Error{Fields: []FieldError{{Code: "nil_pointer", Message: "cannot validate nil pointer"}}}
		}
		rv = rv.Elem()
	}

	// Get the concrete value (dereferenced) for custom validator
	concreteV := rv.Interface()

	// Custom validator runs first (on dereferenced value)
	if cfg.customValidator != nil {
		if err := cfg.customValidator(concreteV); err != nil {
			return coerceToValidationErrors(err, cfg)
		}
	}

	// Run all strategies if requested (use original v to preserve pointer)
	if cfg.runAll {
		return validateAll(v, cfg)
	}

	// Determine strategy (use original v to check interfaces)
	strategy := cfg.strategy
	if strategy == StrategyAuto {
		strategy = determineStrategy(v, cfg)
	}

	// Run single strategy (use original v to preserve pointer for interface validation)
	return validateByStrategy(v, strategy, cfg)
}

// ValidatePartial validates only fields present in the PresenceMap.
// Useful for PATCH requests where only provided fields should be validated.
func ValidatePartial(ctx context.Context, v any, pm PresenceMap, opts ...Option) error {
	opts = append([]Option{WithPresence(pm), WithPartial(true)}, opts...)
	return Validate(ctx, v, opts...)
}

// validateAll runs all applicable validation strategies and aggregates errors.
func validateAll(v any, cfg *config) error {
	var all Error
	strategies := []Strategy{StrategyInterface, StrategyTags, StrategyJSONSchema}
	applied := 0

	for _, strategy := range strategies {
		if !isApplicable(v, strategy, cfg) {
			continue
		}

		applied++
		if err := validateByStrategy(v, strategy, cfg); err != nil {
			all.AddError(err)

			// Check max errors
			if cfg.maxErrors > 0 && len(all.Fields) >= cfg.maxErrors {
				all.Truncated = true
				break
			}
		}
	}

	// If requireAny is true, at least one strategy must have passed
	if cfg.requireAny && applied > 0 && len(all.Fields) == 0 {
		return nil
	}

	if all.HasErrors() {
		all.Sort()
		return &all
	}

	return nil
}

// isApplicable checks if a validation strategy can apply to the value.
func isApplicable(v any, strategy Strategy, cfg *config) bool {
	switch strategy {
	case StrategyInterface:
		// Check if value implements Validator or ValidatorWithContext
		// Check both value and pointer types
		if _, ok := v.(Validator); ok {
			return true
		}
		if cfg.ctx != nil {
			if _, ok := v.(ValidatorWithContext); ok {
				return true
			}
		}
		// Also check if pointer type implements (for pointer receivers)
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr && !rv.IsNil() {
			if rv.Type().Implements(reflect.TypeFor[Validator]()) {
				return true
			}
			if cfg.ctx != nil {
				if rv.Type().Implements(reflect.TypeFor[ValidatorWithContext]()) {
					return true
				}
			}
		}
		// Check if value can be addressed and pointer implements
		if rv.IsValid() && rv.CanAddr() {
			ptrType := rv.Addr().Type()
			if ptrType.Implements(reflect.TypeFor[Validator]()) {
				return true
			}
			if cfg.ctx != nil {
				if ptrType.Implements(reflect.TypeFor[ValidatorWithContext]()) {
					return true
				}
			}
		}
		return false

	case StrategyTags:
		// Tags require a struct type with actual validation tags
		rv := reflect.ValueOf(v)
		for rv.Kind() == reflect.Ptr {
			if rv.IsNil() {
				return false
			}
			rv = rv.Elem()
		}
		if rv.Kind() != reflect.Struct {
			return false
		}
		// Check if struct has any validation tags
		rt := rv.Type()
		for i := 0; i < rt.NumField(); i++ {
			field := rt.Field(i)
			if field.Tag.Get("validate") != "" {
				return true
			}
		}
		return false

	case StrategyJSONSchema:
		// JSON Schema requires a schema to be available
		if cfg.customSchema != "" {
			return true
		}
		if _, ok := v.(JSONSchemaProvider); ok {
			return true
		}
		return false

	default:
		return false
	}
}

// determineStrategy automatically determines the best validation strategy.
func determineStrategy(v any, cfg *config) Strategy {
	// Priority order:
	// 1. Interface validation (Validate/ValidateContext)
	// 2. Tag validation (struct tags)
	// 3. JSON Schema

	if isApplicable(v, StrategyInterface, cfg) {
		return StrategyInterface
	}

	if isApplicable(v, StrategyTags, cfg) {
		return StrategyTags
	}

	if isApplicable(v, StrategyJSONSchema, cfg) {
		return StrategyJSONSchema
	}

	// Default to tags if it's a struct
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return StrategyTags
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Struct {
		return StrategyTags
	}

	return StrategyTags
}

// validateByStrategy dispatches to the appropriate validation function.
func validateByStrategy(v any, strategy Strategy, cfg *config) error {
	switch strategy {
	case StrategyInterface:
		// Use original value (may be pointer) for interface validation
		return validateWithInterface(v, cfg)

	case StrategyTags:
		// Dereference for tag validation (tags work on struct values)
		rv := reflect.ValueOf(v)
		for rv.Kind() == reflect.Ptr && !rv.IsNil() {
			rv = rv.Elem()
		}
		return validateWithTags(rv.Interface(), cfg)

	case StrategyJSONSchema:
		// Dereference for schema validation
		rv := reflect.ValueOf(v)
		for rv.Kind() == reflect.Ptr && !rv.IsNil() {
			rv = rv.Elem()
		}
		return validateWithSchema(rv.Interface(), cfg)

	default:
		return &Error{Fields: []FieldError{{Code: "unknown_strategy", Message: fmt.Sprintf("%v: %d", ErrUnknownValidationStrategy, strategy)}}}
	}
}

// coerceToValidationErrors converts an error to Error.
func coerceToValidationErrors(err error, cfg *config) error {
	if err == nil {
		return nil
	}

	// Already an Error
	if verrs, ok := err.(*Error); ok {
		if cfg.maxErrors > 0 && len(verrs.Fields) > cfg.maxErrors {
			verrs.Fields = verrs.Fields[:cfg.maxErrors]
			verrs.Truncated = true
		}
		verrs.Sort()
		return verrs
	}

	// Already a FieldError
	if fe, ok := err.(FieldError); ok {
		return &Error{Fields: []FieldError{fe}}
	}

	// Generic error - wrap it
	result := &Error{
		Fields: []FieldError{
			{
				Code:    "validation_error",
				Message: err.Error(),
			},
		},
	}

	// Check if it's the sentinel error
	if errors.Is(err, ErrValidation) {
		return result
	}

	return result
}
