package router

import (
	"fmt"
	"net/http"
)

// BindAndValidate binds the request body and validates it.
// This is the most common validation pattern for handlers.
//
// It automatically:
//   - Injects the request context into validation options
//   - Injects the presence map for partial validation
//
// Example:
//
//	var req CreateUserRequest
//	if err := c.BindAndValidate(&req); err != nil {
//	    c.ValidationError(err, 400)
//	    return
//	}
//
// With options:
//
//	if err := c.BindAndValidate(&req,
//	    WithStrategy(ValidationTags),
//	    WithPartial(true),
//	); err != nil {
//	    c.ValidationError(err, 400)
//	    return
//	}
func (c *Context) BindAndValidate(out any, opts ...ValidationOption) error {
	if err := c.BindBody(out); err != nil {
		return fmt.Errorf("binding failed: %w", err)
	}

	allOpts := []ValidationOption{
		WithContext(c.Request.Context()),
		WithPresence(c.Presence()),
	}
	allOpts = append(allOpts, opts...)

	return Validate(out, allOpts...)
}

// BindAndValidateStrict binds JSON with unknown field rejection and validates.
// This is useful for catching typos and API drift early.
//
// Example:
//
//	var req CreateUserRequest
//	if err := c.BindAndValidateStrict(&req); err != nil {
//	    c.ValidationError(err, 400)
//	    return
//	}
func (c *Context) BindAndValidateStrict(out any, opts ...ValidationOption) error {
	if err := c.BindJSONStrict(out); err != nil {
		return err
	}

	allOpts := []ValidationOption{
		WithContext(c.Request.Context()),
		WithPresence(c.Presence()),
		WithDisallowUnknownFields(true),
	}
	allOpts = append(allOpts, opts...)

	return Validate(out, allOpts...)
}

// ValidationError formats and sends a validation error response.
// If the error is a ValidationErrors, it returns structured JSON.
// Otherwise, it returns a simple error message.
//
// Example:
//
//	if err := c.BindAndValidate(&req); err != nil {
//	    c.ValidationError(err, 400)
//	    return
//	}
func (c *Context) ValidationError(err error, status int) {
	if verrs, ok := err.(ValidationErrors); ok {
		c.JSON(status, map[string]any{
			"error":   "validation_failed",
			"details": verrs.Errors,
		})
		return
	}

	c.JSON(status, map[string]string{
		"error": err.Error(),
	})
}

// MustBindAndValidate binds and validates, writing an error response on failure.
// Returns true if validation succeeded, false otherwise.
// This method does NOT panic - it returns a boolean for control flow.
//
// Example:
//
//	var req CreateUserRequest
//	if !c.MustBindAndValidate(&req) {
//	    return // Error already written
//	}
//	// Continue with validated request
func (c *Context) MustBindAndValidate(out any, opts ...ValidationOption) bool {
	if err := c.BindAndValidate(out, opts...); err != nil {
		c.ValidationError(err, 400)
		return false
	}
	return true
}

// ValidationErrorRFC7807 writes an RFC 9457 Problem Details response for validation errors.
// Uses 422 Unprocessable Entity to distinguish semantic validation from parse errors (400).
//
// RFC 9457 guidance: Use 422 when the request is well-formed but contains semantic errors.
// This provides better signals to clients about the nature of the error.
//
// Example:
//
//	if err := c.BindAndValidate(&req); err != nil {
//		return c.ValidationErrorRFC7807(err, router.ProblemTypeValidation)
//	}
func (c *Context) ValidationErrorRFC7807(err error, typeURI string) error {
	if err == nil {
		return fmt.Errorf("ValidationErrorRFC7807 called with nil error")
	}

	// Use 422 for semantic validation failures (not 400)
	p := NewProblemDetail(http.StatusUnprocessableEntity, "Validation Failed").
		WithType(typeURI).
		WithInstance(c.Request.URL.Path).
		WithCause(err) // Chain the original error

	// Handle ValidationErrors specially
	if verrs, ok := err.(ValidationErrors); ok {
		p.WithDetail(fmt.Sprintf("Request validation failed with %d error(s)", len(verrs.Errors)))
		p.WithExtension("errors", verrs.Errors)

		if verrs.Truncated {
			p.WithExtension("truncated", true)
		}
	} else {
		p.WithDetail(err.Error())
	}

	return c.ProblemDetail(p)
}

// MustBindAndValidateRFC7807 combines binding, validation, and RFC 9457 error response.
// Returns true if validation succeeded, false otherwise (with error already written).
//
// Example:
//
//	var req CreateUserRequest
//	if !c.MustBindAndValidateRFC7807(&req, router.ProblemTypeValidation) {
//		return // Error already written
//	}
//	// Continue with validated request
func (c *Context) MustBindAndValidateRFC7807(out any, typeURI string, opts ...ValidationOption) bool {
	if err := c.BindAndValidate(out, opts...); err != nil {
		_ = c.ValidationErrorRFC7807(err, typeURI)
		return false
	}
	return true
}
