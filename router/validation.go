// Package router provides high-performance HTTP routing with comprehensive validation.
//
// # Validation Overview
//
// The validation system supports three strategies that can be used individually or combined:
//   - Interface validation: Custom Validate() or ValidateContext() methods
//   - Tag validation: go-playground/validator struct tags
//   - JSON Schema: JSON Schema validation
//
// # Quick Start
//
// Basic validation with automatic strategy selection:
//
//	var req CreateUserRequest
//	if err := router.Validate(&req); err != nil {
//	    // Handle validation error
//	}
//
// With HTTP context integration:
//
//	func handler(c *router.Context) {
//	    var req CreateUserRequest
//	    if !c.MustBindAndValidate(&req) {
//	        return // Error response already sent
//	    }
//	    // Use validated req
//	}
//
// # Validation Strategies
//
// ## Auto Selection (Default)
//
// Automatically chooses the best strategy in order:
//  1. Interface methods (Validate/ValidateContext) - if implemented
//  2. Struct tags - if struct has validation tags
//  3. JSON Schema - if JSONSchemaProvider implemented or custom schema provided
//
// ## Interface Validation
//
// Implement Validator for custom business logic:
//
//	type User struct {
//	    Email string `json:"email"`
//	}
//
//	func (u *User) Validate() error {
//	    if !strings.Contains(u.Email, "@") {
//	        return errors.New("invalid email")
//	    }
//	    return nil
//	}
//
// Or ValidatorWithContext for request-scoped validation:
//
//	func (u *User) ValidateContext(ctx context.Context) error {
//	    userID := ctx.Value("user_id").(string)
//	    // Use context data for validation
//	    return nil
//	}
//
// ## Tag Validation
//
// Use go-playground/validator struct tags:
//
//	type User struct {
//	    Email    string `json:"email" validate:"required,email"`
//	    Age      int    `json:"age" validate:"min=18,max=120"`
//	    Username string `json:"username" validate:"username"` // Custom tag
//	}
//
// Register custom tags at startup:
//
//	func init() {
//	    router.RegisterTag("custom_tag", func(fl validator.FieldLevel) bool {
//	        return fl.Field().String() == "valid"
//	    })
//	}
//
// ## JSON Schema Validation
//
// Provide schema via JSONSchemaProvider interface:
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
//
// Or provide custom schema:
//
//	router.Validate(&req, router.WithCustomSchema("my-schema", schemaJSON))
//
// # Partial Validation (PATCH Requests)
//
// Validate only fields that were provided:
//
//	func handler(c *router.Context) {
//	    var req UpdateUserRequest
//	    if !c.MustBindAndValidate(&req, router.WithPartial(true)) {
//	        return
//	    }
//	    // Only provided fields were validated
//	}
//
// # Error Handling
//
// Structured validation errors with machine-readable codes:
//
//	err := router.Validate(&req)
//	if verrs, ok := err.(router.ValidationErrors); ok {
//	    for _, e := range verrs.Errors {
//	        fmt.Printf("%s: %s (code: %s)\n", e.Path, e.Message, e.Code)
//	    }
//	}
//
// Check for specific error codes:
//
//	if verrs.HasCode("tag.required") {
//	    // Handle required field error
//	}
//
// # Performance Considerations
//
// The validation system includes several optimizations:
//   - Type interface checking is cached (reduces reflection overhead)
//   - Field path resolution is cached
//   - JSON Schemas are cached with LRU eviction
//   - Configurable limits prevent pathological inputs
//
// Performance characteristics by strategy:
//   - Interface: O(1) method call + custom logic complexity
//   - Tags: O(n) where n = number of validated fields
//   - JSON Schema: O(n) where n = data size, plus schema compilation (cached)
//
// # Thread Safety
//
// All validation functions are safe for concurrent use with these exceptions:
//   - RegisterTag() must be called before first validation (frozen after first use)
//   - Custom validators should be thread-safe
//   - Redactor functions should be thread-safe
//
// # Configuration Options
//
// Available options for fine-tuning validation behavior:
//   - WithStrategy() - Force specific validation strategy
//   - WithPartial() - Enable partial validation mode
//   - WithMaxErrors() - Limit number of errors returned
//   - WithMaxFields() - Limit fields validated in partial mode
//   - WithContext() - Provide context for ValidatorWithContext
//   - WithPresence() - Track which fields are present
//   - WithCustomSchema() - Provide JSON Schema
//   - WithCustomValidator() - Run custom validation first
//   - WithRedactor() - Redact sensitive values in errors
//   - WithFieldNameMapper() - Transform field names in errors
//   - WithRunAll() - Run all strategies and aggregate errors
//   - WithMaxCachedSchemas() - Control schema cache size
//
// See individual option functions for detailed documentation.
package router

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrValidation is a sentinel error for validation failures.
// Use errors.Is(err, ErrValidation) to check if an error is a validation error.
var ErrValidation = errors.New("validation")

// ValidationStrategy defines the validation approach.
type ValidationStrategy int

const (
	// ValidationAuto automatically selects the best validation strategy.
	// Order: 1) Validate()/ValidateContext() methods, 2) struct tags, 3) JSON Schema.
	ValidationAuto ValidationStrategy = iota

	// ValidationTags uses go-playground/validator struct tags for validation.
	ValidationTags

	// ValidationJSONSchema uses JSON Schema for validation.
	ValidationJSONSchema

	// ValidationInterface uses custom Validate() or ValidateContext() methods.
	ValidationInterface
)

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

// PresenceMap tracks which fields are present in the request body.
// Keys are normalized dot paths (e.g., "items.2.price"), values are booleans.
//
// PresenceMap is used for partial update validation (PATCH requests),
// where only present fields should be validated, while absent fields
// should be ignored even if they have "required" constraints.
type PresenceMap map[string]bool

// Has returns true if the exact path is present.
func (pm PresenceMap) Has(path string) bool {
	return pm != nil && pm[path]
}

// HasPrefix returns true if any path with the given prefix is present.
// This is useful for checking if a nested object or array element is present.
func (pm PresenceMap) HasPrefix(prefix string) bool {
	if pm == nil {
		return false
	}
	prefixDot := prefix + "."
	for path := range pm {
		if path == prefix || strings.HasPrefix(path, prefixDot) {
			return true
		}
	}
	return false
}

// LeafPaths returns paths that aren't prefixes of others.
// This is useful for partial validation where we only want to validate
// the leaf fields that were actually provided, not their parent objects.
//
// Example:
//   - If presence contains "address" and "address.city", only "address.city" is a leaf.
//   - If presence contains "items.0" and "items.0.name", only "items.0.name" is a leaf.
func (pm PresenceMap) LeafPaths() []string {
	if pm == nil {
		return nil
	}

	paths := make([]string, 0, len(pm))
	for p := range pm {
		paths = append(paths, p)
	}

	// Sort to process in order
	sort.Strings(paths)

	leaves := make([]string, 0, len(paths))
	for i, path := range paths {
		isLeaf := true
		pathDot := path + "."

		// Check if any later path is a child
		for j := i + 1; j < len(paths); j++ {
			if strings.HasPrefix(paths[j], pathDot) {
				isLeaf = false
				break
			}
		}

		if isLeaf {
			leaves = append(leaves, path)
		}
	}

	return leaves
}

// Redactor function for sensitive fields.
// Returns true if the field at the given path should be redacted.
//
// Example:
//
//	redactor := func(path string) bool {
//	    return strings.Contains(path, "password") || strings.Contains(path, "token")
//	}
type Redactor func(path string) bool

// FieldError represents a single validation error for a specific field.
type FieldError struct {
	Path    string         `json:"path"`           // JSON path (e.g., "items.2.price")
	Code    string         `json:"code"`           // Stable code (e.g., "tag.required", "schema.type")
	Message string         `json:"message"`        // Human-readable message
	Meta    map[string]any `json:"meta,omitempty"` // Additional metadata (tag, param, value, etc.)
}

// Error returns a formatted error message.
func (e FieldError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// Unwrap returns ErrValidation for errors.Is/errors.As compatibility.
func (e FieldError) Unwrap() error {
	return ErrValidation
}

// ValidationErrors collection of field errors.
// This type implements error and can be used with errors.Is/errors.As.
type ValidationErrors struct {
	Errors    []FieldError `json:"errors"`              // List of field errors
	Truncated bool         `json:"truncated,omitempty"` // True if errors were truncated due to maxErrors limit
}

// Error returns a formatted error message.
func (v ValidationErrors) Error() string {
	if len(v.Errors) == 0 {
		return ""
	}
	if len(v.Errors) == 1 {
		return v.Errors[0].Error()
	}

	suffix := ""
	if v.Truncated {
		suffix = " (truncated)"
	}

	var msgs []string
	for _, err := range v.Errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("validation failed: %s%s", strings.Join(msgs, "; "), suffix)
}

// Unwrap returns ErrValidation for errors.Is/errors.As compatibility.
func (v ValidationErrors) Unwrap() error {
	return ErrValidation
}

// Add adds a new field error to the collection.
func (v *ValidationErrors) Add(path, code, message string, meta map[string]any) {
	v.Errors = append(v.Errors, FieldError{
		Path:    path,
		Code:    code,
		Message: message,
		Meta:    meta,
	})
}

// AddError adds an error to the collection, handling different error types.
func (v *ValidationErrors) AddError(err error) {
	if err == nil {
		return
	}

	if fe, ok := err.(FieldError); ok {
		v.Errors = append(v.Errors, fe)
		return
	}

	if ve, ok := err.(ValidationErrors); ok {
		v.Errors = append(v.Errors, ve.Errors...)
		if ve.Truncated {
			v.Truncated = true
		}
		return
	}

	v.Errors = append(v.Errors, FieldError{
		Code:    "validation_error",
		Message: err.Error(),
	})
}

// HasErrors returns true if there are any errors.
func (v ValidationErrors) HasErrors() bool {
	return len(v.Errors) > 0
}

// HasCode returns true if any error has the given code.
func (v ValidationErrors) HasCode(code string) bool {
	for _, e := range v.Errors {
		if e.Code == code {
			return true
		}
	}
	return false
}

// Sort sorts errors by path, then by code.
func (v *ValidationErrors) Sort() {
	sort.Slice(v.Errors, func(i, j int) bool {
		if v.Errors[i].Path != v.Errors[j].Path {
			return v.Errors[i].Path < v.Errors[j].Path
		}
		return v.Errors[i].Code < v.Errors[j].Code
	})
}

// contextKey is a type for context keys to avoid collisions.
type contextKey int

const (
	// contextKeyRawJSON is used to store raw JSON body in request context
	// for optimization in JSON Schema validation.
	contextKeyRawJSON contextKey = iota
)
