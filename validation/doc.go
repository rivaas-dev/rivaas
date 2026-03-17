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

// Package validation provides flexible, multi-strategy validation for Go structs.
//
// # Getting Started
//
// The simplest way to use this package is with the package-level [Validate] function:
//
//	type User struct {
//		Email string `json:"email" validate:"required,email"`
//		Age   int    `json:"age" validate:"min=18"`
//	}
//
//	user := User{Email: "invalid", Age: 15}
//	if err := validation.Validate(ctx, &user); err != nil {
//		var verr *validation.Error
//		if errors.As(err, &verr) {
//			for _, fieldErr := range verr.Fields {
//				fmt.Printf("%s: %s\n", fieldErr.Path, fieldErr.Message)
//			}
//		}
//	}
//
// For more control, create an [Engine] with [New] or [MustNew]:
//
//	engine := validation.MustNew(
//		validation.WithRedactor(sensitiveFieldRedactor),
//		validation.WithMaxErrors(10),
//		validation.WithCustomTag("phone", phoneValidator),
//	)
//
//	if err := engine.Validate(ctx, &user); err != nil {
//		// Handle validation errors
//	}
//
// # Validation Strategies
//
// The package supports three validation strategies:
//
//  1. Struct Tags - Using go-playground/validator tags (e.g., `validate:"required,email"`)
//  2. JSON Schema - RFC-compliant JSON Schema validation via [JSONSchemaProvider] interface
//  3. Custom Interfaces - Implement [Validator] or [ValidatorWithContext] for custom validation logic
//
// The package automatically selects the best strategy based on the value type, or you can
// explicitly choose a strategy using [WithStrategy].
//
// # Partial Validation
//
// For PATCH requests where only some fields are provided, use [ValidatePartial]:
//
//	presence, _ := validation.ComputePresence(rawJSON)
//	err := engine.ValidatePartial(ctx, &user, presence)
//
// # Custom Validation Interface
//
// Implement [Validator] for custom validation logic:
//
//	type User struct {
//		Email string
//	}
//
//	func (u *User) Validate() error {
//		if !strings.Contains(u.Email, "@") {
//			return errors.New("email must contain @")
//		}
//		return nil
//	}
//
//	// validation.Validate will automatically call u.Validate()
//	err := validation.Validate(ctx, &user)
//
// When the method is on a pointer receiver, pass a pointer to [Validate] or [Engine.Validate].
//
// For context-aware validation, implement [ValidatorWithContext]:
//
//	func (u *User) ValidateContext(ctx context.Context) error {
//		// Access request-scoped data from context
//		tenant := ctx.Value("tenant").(string)
//		// Apply tenant-specific validation rules
//		return nil
//	}
//
// # Sentinel errors
//
// Use errors.Is(err, ErrValidation) for validation failures. For specific cases (nil value,
// invalid value, unknown strategy) see ErrCannotValidateNilValue, ErrCannotValidateInvalidValue,
// and ErrUnknownValidationStrategy. These are the single source of truth for validation sentinels.
//
// # Thread Safety
//
// [Engine] instances are safe for concurrent use by multiple goroutines.
// Package-level [Validate] and [ValidatePartial] use [DefaultEngine], which is
// lazily initialized on first use (thread-safe). For test isolation or multiple
// engines in the same process, create an Engine with [New] or [MustNew] and use
// [Engine.Validate] instead of the package-level functions.
//
// # Security
//
// The package includes protections against:
//
//   - Stack overflow from deeply nested structures (max depth: 100)
//   - Unbounded memory usage (configurable limits on errors and fields)
//   - Sensitive data exposure (redaction support via [WithRedactor])
//
// # Standalone Usage
//
// This package can be used independently without the full Rivaas framework.
// Either use the package-level API (which uses [DefaultEngine]) or create an
// explicit Engine:
//
//	import "rivaas.dev/validation"
//
//	// Zero config: uses DefaultEngine (lazily initialized)
//	err := validation.Validate(ctx, &user)
//
//	// Or create an engine for custom configuration
//	engine := validation.MustNew(validation.WithMaxErrors(10))
//	err := engine.Validate(ctx, &user)
package validation
