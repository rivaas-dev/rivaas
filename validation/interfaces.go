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

import "context"

// ValidatorInterface is the interface for custom validation methods.
// When implemented by a struct, it causes Validate() to be called during validation.
//
// Note: This interface is named ValidatorInterface to avoid confusion with the
// [Validator] struct which is the main validation engine.
//
// Example:
//
//	type User struct {
//	    Email string `json:"email"`
//	}
//
//	func (u *User) Validate() error {
//	    if !strings.Contains(u.Email, "@") {
//	        return errors.New("email must contain @")
//	    }
//	    return nil
//	}
type ValidatorInterface interface {
	Validate() error
}

// ValidatorWithContext interface for context-aware validation methods.
// It is preferred over ValidatorInterface when a context is available,
// as it allows for tenant-specific rules, request-scoped data, etc.
//
// Example:
//
//	type User struct {
//	    Email string `json:"email"`
//	}
//
//	func (u *User) ValidateContext(ctx context.Context) error {
//	    userID := ctx.Value("user_id").(string)
//	    // Use context data for validation (e.g., tenant-specific rules)
//	    return nil
//	}
type ValidatorWithContext interface {
	ValidateContext(context.Context) error
}

// JSONSchemaProvider interface for types that provide their own JSON Schema.
// When implemented by a struct, it causes the returned schema to be used for validation.
//
// Example:
//
//	type User struct {
//	    Email string `json:"email"`
//	}
//
//	func (u *User) JSONSchema() (id string, schema string) {
//	    return "user-v1", `{
//	        "type": "object",
//	        "properties": {
//	            "email": {"type": "string", "format": "email"}
//	        },
//	        "required": ["email"]
//	    }`
//	}
type JSONSchemaProvider interface {
	JSONSchema() (id string, schema string)
}
