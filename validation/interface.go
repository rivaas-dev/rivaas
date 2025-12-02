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
	"errors"
	"fmt"
	"reflect"
)

// errNotImplemented is returned internally when a type doesn't implement
// [ValidatorInterface] or [ValidatorWithContext].
var errNotImplemented = fmt.Errorf("validator not implemented")

// validateWithInterface validates using [ValidatorInterface] or [ValidatorWithContext] methods.
// This implements [StrategyInterface] validation.
func (v *Validator) validateWithInterface(ctx context.Context, val any, cfg *config) error {
	// Prefer ValidatorWithContext if context is available
	if ctx != nil {
		if validator, ok := val.(ValidatorWithContext); ok {
			if err := validator.ValidateContext(ctx); err != nil {
				return v.coerceToValidationErrors(err, cfg)
			}
			return nil
		}

		// Try pointer receiver with context
		if err := v.callValidatorWithContext(ctx, val); err != nil {
			if !errors.Is(err, errNotImplemented) {
				return v.coerceToValidationErrors(err, cfg)
			}
		} else {
			return nil
		}
	}

	// Try ValidatorInterface interface
	if validator, ok := val.(ValidatorInterface); ok {
		if err := validator.Validate(); err != nil {
			return v.coerceToValidationErrors(err, cfg)
		}
		return nil
	}

	// Try pointer receiver
	if err := v.callValidator(val); err != nil {
		if !errors.Is(err, errNotImplemented) {
			return v.coerceToValidationErrors(err, cfg)
		}
	} else {
		return nil
	}

	// No validator found
	return nil
}

// callValidator calls Validate() method using reflection to support both value and pointer receivers.
func (v *Validator) callValidator(val any) error {
	rv := reflect.ValueOf(val)
	rt := reflect.TypeOf(val)

	// Try direct call
	if v.typeImplementsValidator(rt) {
		method := rv.MethodByName("Validate")
		if method.IsValid() {
			results := method.Call(nil)
			if len(results) > 0 && !results[0].IsNil() {
				if err, ok := results[0].Interface().(error); ok {
					return err
				}
			}
			return nil
		}
	}

	// Try pointer receiver
	if rv.CanAddr() {
		ptrVal := rv.Addr()
		ptrType := ptrVal.Type()
		if v.typeImplementsValidator(ptrType) {
			method := ptrVal.MethodByName("Validate")
			if method.IsValid() {
				results := method.Call(nil)
				if len(results) > 0 && !results[0].IsNil() {
					if err, ok := results[0].Interface().(error); ok {
						return err
					}
				}
				return nil
			}
		}
	}

	// Try value receiver on pointer
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		elemVal := rv.Elem()
		elemType := elemVal.Type()
		if v.typeImplementsValidator(elemType) {
			method := elemVal.MethodByName("Validate")
			if method.IsValid() {
				results := method.Call(nil)
				if len(results) > 0 && !results[0].IsNil() {
					if err, ok := results[0].Interface().(error); ok {
						return err
					}
				}
				return nil
			}
		}
	}

	return errNotImplemented
}

// callValidatorWithContext calls the ValidateContext() method using reflection.
// It supports both value and pointer receivers for [ValidatorWithContext].
func (v *Validator) callValidatorWithContext(ctx context.Context, val any) error {
	rv := reflect.ValueOf(val)
	rt := reflect.TypeOf(val)

	// Try direct call
	if v.typeImplementsValidatorWithContext(rt) {
		method := rv.MethodByName("ValidateContext")
		if method.IsValid() {
			ctxVal := reflect.ValueOf(ctx)
			results := method.Call([]reflect.Value{ctxVal})
			if len(results) > 0 && !results[0].IsNil() {
				if err, ok := results[0].Interface().(error); ok {
					return err
				}
			}
			return nil
		}
	}

	// Try pointer receiver
	if rv.CanAddr() {
		ptrVal := rv.Addr()
		ptrType := ptrVal.Type()
		if v.typeImplementsValidatorWithContext(ptrType) {
			method := ptrVal.MethodByName("ValidateContext")
			if method.IsValid() {
				ctxVal := reflect.ValueOf(ctx)
				results := method.Call([]reflect.Value{ctxVal})
				if len(results) > 0 && !results[0].IsNil() {
					if err, ok := results[0].Interface().(error); ok {
						return err
					}
				}
				return nil
			}
		}
	}

	// Try value receiver on pointer
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		elemVal := rv.Elem()
		elemType := elemVal.Type()
		if v.typeImplementsValidatorWithContext(elemType) {
			method := elemVal.MethodByName("ValidateContext")
			if method.IsValid() {
				ctxVal := reflect.ValueOf(ctx)
				results := method.Call([]reflect.Value{ctxVal})
				if len(results) > 0 && !results[0].IsNil() {
					if err, ok := results[0].Interface().(error); ok {
						return err
					}
				}
				return nil
			}
		}
	}

	return errNotImplemented
}

// coerceToValidationErrors converts an error to [*Error].
// It handles [FieldError], [Error], and generic errors.
func (v *Validator) coerceToValidationErrors(err error, cfg *config) error {
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
