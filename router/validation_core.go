package router

import (
	"fmt"
	"reflect"
)

// Validate validates a value using the specified validation strategy.
// This is the main entry point for validation.
//
// Example:
//
//	var req CreateUserRequest
//	if err := Validate(&req); err != nil {
//	    // Handle validation error
//	}
//
// With options:
//
//	if err := Validate(&req,
//	    WithStrategy(ValidationTags),
//	    WithPartial(true),
//	    WithPresence(presenceMap),
//	); err != nil {
//	    // Handle validation error
//	}
func Validate(v any, opts ...ValidationOption) error {
	if v == nil {
		return ErrCannotValidateNilValue
	}

	cfg := newValidationConfig(opts...)

	// Handle nil pointers and invalid values
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return ErrCannotValidateInvalidValue
	}

	// Check for nil pointers (but preserve pointer for interface validation)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return newValidationError("", "nil_pointer", "cannot validate nil pointer", nil)
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
	if strategy == ValidationAuto {
		strategy = determineStrategy(v, cfg)
	}

	// Run single strategy (use original v to preserve pointer for interface validation)
	return validateByStrategy(v, strategy, cfg)
}

// validateAll runs all applicable validation strategies and aggregates errors.
func validateAll(v any, cfg *validationConfig) error {
	var all ValidationErrors
	strategies := []ValidationStrategy{ValidationInterface, ValidationTags, ValidationJSONSchema}
	applied := 0

	for _, strategy := range strategies {
		if !isApplicable(v, strategy, cfg) {
			continue
		}

		applied++
		if err := validateByStrategy(v, strategy, cfg); err != nil {
			if verrs, ok := err.(ValidationErrors); ok {
				all.Errors = append(all.Errors, verrs.Errors...)
				if verrs.Truncated {
					all.Truncated = true
				}
			} else {
				all.AddError(err)
			}

			// Check max errors
			if cfg.maxErrors > 0 && len(all.Errors) >= cfg.maxErrors {
				all.Truncated = true
				break
			}
		}
	}

	// If requireAny is true, at least one strategy must have passed
	if cfg.requireAny && applied > 0 && len(all.Errors) == 0 {
		return nil
	}

	if all.HasErrors() {
		all.Sort()
		return all
	}

	return nil
}

// isApplicable checks if a validation strategy can apply to the value.
func isApplicable(v any, strategy ValidationStrategy, cfg *validationConfig) bool {
	switch strategy {
	case ValidationInterface:
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

	case ValidationTags:
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

	case ValidationJSONSchema:
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
func determineStrategy(v any, cfg *validationConfig) ValidationStrategy {
	// Priority order:
	// 1. Interface validation (Validate/ValidateContext)
	// 2. Tag validation (struct tags)
	// 3. JSON Schema

	if isApplicable(v, ValidationInterface, cfg) {
		return ValidationInterface
	}

	if isApplicable(v, ValidationTags, cfg) {
		return ValidationTags
	}

	if isApplicable(v, ValidationJSONSchema, cfg) {
		return ValidationJSONSchema
	}

	// Default to tags if it's a struct
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ValidationTags
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Struct {
		return ValidationTags
	}

	return ValidationTags
}

// validateByStrategy dispatches to the appropriate validation function.
func validateByStrategy(v any, strategy ValidationStrategy, cfg *validationConfig) error {
	switch strategy {
	case ValidationInterface:
		// Use original value (may be pointer) for interface validation
		return validateWithInterface(v, cfg)

	case ValidationTags:
		// Dereference for tag validation (tags work on struct values)
		rv := reflect.ValueOf(v)
		for rv.Kind() == reflect.Ptr && !rv.IsNil() {
			rv = rv.Elem()
		}
		return validateWithTags(rv.Interface(), cfg)

	case ValidationJSONSchema:
		// Dereference for schema validation
		rv := reflect.ValueOf(v)
		for rv.Kind() == reflect.Ptr && !rv.IsNil() {
			rv = rv.Elem()
		}
		return validateWithSchema(rv.Interface(), cfg)

	default:
		return fmt.Errorf("%w: %d", ErrUnknownValidationStrategy, strategy)
	}
}

// coerceToValidationErrors converts an error to ValidationErrors.
func coerceToValidationErrors(err error, cfg *validationConfig) error {
	if err == nil {
		return nil
	}

	// Already a ValidationErrors
	if verrs, ok := err.(ValidationErrors); ok {
		if cfg.maxErrors > 0 && len(verrs.Errors) > cfg.maxErrors {
			verrs.Errors = verrs.Errors[:cfg.maxErrors]
			verrs.Truncated = true
		}
		verrs.Sort()
		return verrs
	}

	// Already a FieldError
	if fe, ok := err.(FieldError); ok {
		return ValidationErrors{Errors: []FieldError{fe}}
	}

	// Generic error - wrap it
	result := ValidationErrors{
		Errors: []FieldError{
			{
				Code:    "validation_error",
				Message: err.Error(),
			},
		},
	}

	return result
}

// newValidationError creates a new validation error.
func newValidationError(path, code, message string, meta map[string]any) ValidationErrors {
	return ValidationErrors{
		Errors: []FieldError{
			{
				Path:    path,
				Code:    code,
				Message: message,
				Meta:    meta,
			},
		},
	}
}
