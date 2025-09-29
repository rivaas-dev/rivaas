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
// # Validation Strategies
//
// The package supports three validation strategies:
//
//  1. Struct Tags - Using go-playground/validator tags (e.g., `validate:"required,email"`)
//  2. JSON Schema - RFC-compliant JSON Schema validation via JSONSchemaProvider interface
//  3. Custom Interfaces - Implement Validator or ValidatorWithContext for custom validation logic
//
// The package automatically selects the best strategy based on the value type, or you can
// explicitly choose a strategy using WithStrategy().
//
// # Usage
//
// Basic validation with struct tags:
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
// Partial validation for PATCH requests:
//
//	presence := validation.ComputePresence(rawJSON)
//	err := validation.ValidatePartial(ctx, &user, presence)
//
// Custom validation with interface:
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
//	err := validation.Validate(ctx, &user)
//
// # Thread Safety
//
// All validation functions are safe for concurrent use. Custom validator registration
// must happen before first use and is then frozen for thread safety.
//
// # Security
//
// The package includes protections against:
//
//   - Stack overflow from deeply nested structures (max depth: 100)
//   - Unbounded memory usage (configurable limits on errors and fields)
//   - Sensitive data exposure (redaction support via WithRedactor)
package validation
