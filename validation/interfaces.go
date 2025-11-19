package validation

import "context"

// Validator interface for custom validation methods.
// If a struct implements this interface, Validate() will be called during validation.
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
type Validator interface {
	Validate() error
}

// ValidatorWithContext interface for context-aware validation methods.
// This interface is preferred over Validator when a context is available,
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
// If a struct implements this interface, the returned schema will be used for validation.
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
