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
	"sync"
)

var (
	validatorTypeCache sync.Map // map[reflect.Type]bool

	validatorWithContextTypeCache sync.Map // map[reflect.Type]bool
)

var errNotImplemented = fmt.Errorf("validator not implemented")

// validateWithInterface validates using custom Validate() or ValidateContext() methods.
func validateWithInterface(v any, cfg *config) error {
	// Prefer ValidatorWithContext if context is available
	if cfg.ctx != nil {
		if validator, ok := v.(ValidatorWithContext); ok {
			if err := validator.ValidateContext(cfg.ctx); err != nil {
				return coerceToValidationErrors(err, cfg)
			}
			return nil
		}

		// Try pointer receiver with context
		if err := callValidatorWithContext(cfg.ctx, v); err != nil {
			if !errors.Is(err, errNotImplemented) {
				return coerceToValidationErrors(err, cfg)
			}
		} else {
			return nil
		}
	}

	// Try Validator interface
	if validator, ok := v.(Validator); ok {
		if err := validator.Validate(); err != nil {
			return coerceToValidationErrors(err, cfg)
		}
		return nil
	}

	// Try pointer receiver
	if err := callValidator(v); err != nil {
		if !errors.Is(err, errNotImplemented) {
			return coerceToValidationErrors(err, cfg)
		}
	} else {
		return nil
	}

	// No validator found
	return nil
}

// typeImplementsValidator checks if a type implements Validator interface.
func typeImplementsValidator(t reflect.Type) bool {
	if cached, ok := validatorTypeCache.Load(t); ok {
		if result, ok := cached.(bool); ok {
			return result
		}
	}

	implements := t.Implements(reflect.TypeFor[Validator]())

	actual, loaded := validatorTypeCache.LoadOrStore(t, implements)
	if loaded {
		// Another goroutine stored first, use their result
		if result, ok := actual.(bool); ok {
			return result
		}
	}
	return implements
}

// callValidator calls Validate() method using reflection to support both value and pointer receivers.
func callValidator(v any) error {
	rv := reflect.ValueOf(v)
	rt := reflect.TypeOf(v)

	// Try direct call
	if typeImplementsValidator(rt) {
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
		if typeImplementsValidator(ptrType) {
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
		if typeImplementsValidator(elemType) {
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

// typeImplementsValidatorWithContext checks if a type implements ValidatorWithContext interface.
func typeImplementsValidatorWithContext(t reflect.Type) bool {
	if cached, ok := validatorWithContextTypeCache.Load(t); ok {
		if result, ok := cached.(bool); ok {
			return result
		}
	}

	implements := t.Implements(reflect.TypeFor[ValidatorWithContext]())

	actual, loaded := validatorWithContextTypeCache.LoadOrStore(t, implements)
	if loaded {
		// Another goroutine stored first, use their result
		if result, ok := actual.(bool); ok {
			return result
		}
	}
	return implements
}

// callValidatorWithContext calls ValidateContext() method using reflection.
func callValidatorWithContext(ctx context.Context, v any) error {
	rv := reflect.ValueOf(v)
	rt := reflect.TypeOf(v)

	// Try direct call
	if typeImplementsValidatorWithContext(rt) {
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
		if typeImplementsValidatorWithContext(ptrType) {
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
		if typeImplementsValidatorWithContext(elemType) {
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
