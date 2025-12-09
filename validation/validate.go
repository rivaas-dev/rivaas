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

package validation

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// Package-level validator state for convenience functions [Validate] and [ValidatePartial].
var (
	defaultValidator     *Validator
	defaultValidatorOnce sync.Once
)

// getDefaultValidator returns the default [Validator], creating it if necessary.
func getDefaultValidator() *Validator {
	defaultValidatorOnce.Do(func() {
		defaultValidator = MustNew()
	})

	return defaultValidator
}

// Validate validates a value using the default [Validator].
// For customized validation, create a Validator with [New] or [MustNew].
//
// Validate returns nil if validation passes, or an [*Error] if validation fails.
// The error can be type-asserted to *Error for structured field errors.
//
// Parameters:
//   - ctx: Context passed to [ValidatorWithContext] implementations
//   - v: The value to validate (typically a pointer to a struct)
//   - opts: Optional per-call configuration (see [Option])
//
// Example:
//
//	var req CreateUserRequest
//	if err := validation.Validate(ctx, &req); err != nil {
//	    var verr *validation.Error
//	    if errors.As(err, &verr) {
//	        // Handle structured validation errors
//	    }
//	}
//
// With options:
//
//	if err := validation.Validate(ctx, &req,
//	    validation.WithStrategy(StrategyTags),
//	    validation.WithPartial(true),
//	    validation.WithPresence(presenceMap),
//	); err != nil {
//	    // Handle validation error
//	}
func Validate(ctx context.Context, v any, opts ...Option) error {
	return getDefaultValidator().Validate(ctx, v, opts...)
}

// ValidatePartial validates only fields present in the [PresenceMap] using the default [Validator].
// ValidatePartial is useful for PATCH requests where only provided fields should be validated.
// Use [ComputePresence] to create a PresenceMap from raw JSON.
func ValidatePartial(ctx context.Context, v any, pm PresenceMap, opts ...Option) error {
	return getDefaultValidator().ValidatePartial(ctx, v, pm, opts...)
}

// Validate validates a value using this validator's configuration.
//
// Validate returns nil if validation passes, or an [*Error] if validation fails.
// The error can be type-asserted to *Error for structured field errors.
// Per-call options override the validator's base configuration.
//
// Parameters:
//   - ctx: Context passed to [ValidatorWithContext] implementations
//   - val: The value to validate (typically a pointer to a struct)
//   - opts: Optional per-call configuration overrides (see [Option])
//
// Example:
//
//	validator := validation.MustNew(validation.WithMaxErrors(10))
//
//	if err := validator.Validate(ctx, &req); err != nil {
//	    var verr *validation.Error
//	    if errors.As(err, &verr) {
//	        // Handle structured validation errors
//	    }
//	}
func (v *Validator) Validate(ctx context.Context, val any, opts ...Option) error {
	if val == nil {
		return &Error{Fields: []FieldError{{Code: "nil", Message: ErrCannotValidateNilValue.Error()}}}
	}

	// Apply per-call options on top of validator's base config
	cfg := applyOptions(v.cfg, opts...)

	// Use context from config if explicitly set via WithContext, otherwise use the ctx parameter
	if cfg.ctx != nil {
		ctx = cfg.ctx
	}

	// Handle nil pointers and invalid values
	rv := reflect.ValueOf(val)
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
			return v.coerceToValidationErrors(err, cfg)
		}
	}

	// Run all strategies if requested (use original val to preserve pointer)
	if cfg.runAll {
		return v.validateAll(ctx, val, cfg)
	}

	// Determine strategy (use original val to check interfaces)
	strategy := cfg.strategy
	if strategy == StrategyAuto {
		strategy = v.determineStrategy(ctx, val, cfg)
	}

	// Run single strategy (use original val to preserve pointer for interface validation)
	return v.validateByStrategy(ctx, val, strategy, cfg)
}

// ValidatePartial validates only fields present in the [PresenceMap].
// It is useful for PATCH requests where only provided fields should be validated.
// Use [ComputePresence] to create a PresenceMap from raw JSON.
func (v *Validator) ValidatePartial(ctx context.Context, val any, pm PresenceMap, opts ...Option) error {
	opts = append([]Option{WithPresence(pm), WithPartial(true)}, opts...)
	return v.Validate(ctx, val, opts...)
}

// validateAll runs all applicable validation strategies and aggregates errors into an [*Error].
func (v *Validator) validateAll(ctx context.Context, val any, cfg *config) error {
	var all Error
	strategies := []Strategy{StrategyInterface, StrategyTags, StrategyJSONSchema}
	applied := 0

	for _, strategy := range strategies {
		if !v.isApplicable(ctx, val, strategy, cfg) {
			continue
		}

		applied++
		if err := v.validateByStrategy(ctx, val, strategy, cfg); err != nil {
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

// isApplicable checks if a validation [Strategy] can apply to the value.
func (v *Validator) isApplicable(ctx context.Context, val any, strategy Strategy, cfg *config) bool {
	switch strategy {
	case StrategyInterface:
		// Check if value implements ValidatorInterface or ValidatorWithContext
		// Check both value and pointer types
		if _, ok := val.(ValidatorInterface); ok {
			return true
		}
		if ctx != nil {
			if _, ok := val.(ValidatorWithContext); ok {
				return true
			}
		}
		// Also check if pointer type implements (for pointer receivers)
		rv := reflect.ValueOf(val)
		if rv.Kind() == reflect.Ptr && !rv.IsNil() {
			if rv.Type().Implements(reflect.TypeFor[ValidatorInterface]()) {
				return true
			}
			if ctx != nil {
				if rv.Type().Implements(reflect.TypeFor[ValidatorWithContext]()) {
					return true
				}
			}
		}
		// Check if value can be addressed and pointer implements
		if rv.IsValid() && rv.CanAddr() {
			ptrType := rv.Addr().Type()
			if ptrType.Implements(reflect.TypeFor[ValidatorInterface]()) {
				return true
			}
			if ctx != nil {
				if ptrType.Implements(reflect.TypeFor[ValidatorWithContext]()) {
					return true
				}
			}
		}

		return false

	case StrategyTags:
		// Tags require a struct type with actual validation tags
		rv := reflect.ValueOf(val)
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
		for i := range rt.NumField() {
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
		if _, ok := val.(JSONSchemaProvider); ok {
			return true
		}

		return false

	default:
		return false
	}
}

// determineStrategy automatically determines the best validation strategy.
func (v *Validator) determineStrategy(ctx context.Context, val any, cfg *config) Strategy {
	// Priority order:
	// 1. Interface validation (Validate/ValidateContext)
	// 2. Tag validation (struct tags)
	// 3. JSON Schema

	if v.isApplicable(ctx, val, StrategyInterface, cfg) {
		return StrategyInterface
	}

	if v.isApplicable(ctx, val, StrategyTags, cfg) {
		return StrategyTags
	}

	if v.isApplicable(ctx, val, StrategyJSONSchema, cfg) {
		return StrategyJSONSchema
	}

	// Default to tags if it's a struct
	rv := reflect.ValueOf(val)
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

// validateByStrategy dispatches to the appropriate validation function based on [Strategy].
func (v *Validator) validateByStrategy(ctx context.Context, val any, strategy Strategy, cfg *config) error {
	switch strategy {
	case StrategyInterface:
		// Use original value (may be pointer) for interface validation
		return v.validateWithInterface(ctx, val, cfg)

	case StrategyTags:
		// Dereference for tag validation (tags work on struct values)
		rv := reflect.ValueOf(val)
		for rv.Kind() == reflect.Ptr && !rv.IsNil() {
			rv = rv.Elem()
		}

		return v.validateWithTags(rv.Interface(), cfg)

	case StrategyJSONSchema:
		// Dereference for schema validation
		rv := reflect.ValueOf(val)
		for rv.Kind() == reflect.Ptr && !rv.IsNil() {
			rv = rv.Elem()
		}

		return v.validateWithSchema(ctx, rv.Interface(), cfg)

	default:
		return &Error{Fields: []FieldError{{Code: "unknown_strategy", Message: fmt.Sprintf("%v: %d", ErrUnknownValidationStrategy, strategy)}}}
	}
}
