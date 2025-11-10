package router

import (
	"fmt"
	"net/http"

	"rivaas.dev/router/validation"
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
//	    validation.WithStrategy(validation.StrategyTags),
//	    validation.WithPartial(true),
//	); err != nil {
//	    c.ValidationError(err, 400)
//	    return
//	}
func (c *Context) BindAndValidate(out any, opts ...validation.Option) error {
	if err := c.BindBody(out); err != nil {
		return fmt.Errorf("binding failed: %w", err)
	}

	ctx := c.Request.Context()
	// Inject raw JSON if available
	if c.bindingMeta != nil && len(c.bindingMeta.rawBody) > 0 {
		ctx = validation.InjectRawJSONCtx(ctx, c.bindingMeta.rawBody)
	}

	allOpts := []validation.Option{
		validation.WithContext(ctx),
	}
	if pm := c.Presence(); pm != nil {
		allOpts = append(allOpts, validation.WithPresence(pm))
	}
	allOpts = append(allOpts, opts...)

	if verr := validation.Validate(ctx, out, allOpts...); verr != nil {
		return verr
	}
	return nil
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
func (c *Context) BindAndValidateStrict(out any, opts ...validation.Option) error {
	if err := c.BindJSONStrict(out); err != nil {
		return err
	}

	ctx := c.Request.Context()
	// Inject raw JSON if available
	if c.bindingMeta != nil && len(c.bindingMeta.rawBody) > 0 {
		ctx = validation.InjectRawJSONCtx(ctx, c.bindingMeta.rawBody)
	}

	allOpts := []validation.Option{
		validation.WithContext(ctx),
		validation.WithDisallowUnknownFields(true),
	}
	if pm := c.Presence(); pm != nil {
		allOpts = append(allOpts, validation.WithPresence(pm))
	}
	allOpts = append(allOpts, opts...)

	if verr := validation.Validate(ctx, out, allOpts...); verr != nil {
		return verr
	}
	return nil
}

// ValidationError formats and sends a validation error response.
// If the error is a validation.Error, it returns structured JSON.
// Otherwise, it returns a simple error message.
//
// Example:
//
//	if err := c.BindAndValidate(&req); err != nil {
//	    c.ValidationError(err, 400)
//	    return
//	}
func (c *Context) ValidationError(err error, status int) {
	if verr, ok := err.(*validation.Error); ok {
		c.JSON(status, map[string]any{
			"error":   "validation_failed",
			"details": verr.Fields,
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
func (c *Context) MustBindAndValidate(out any, opts ...validation.Option) bool {
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
		return ErrValidationErrorNil
	}

	// Use 422 for semantic validation failures (not 400)
	p := NewProblemDetail(http.StatusUnprocessableEntity, "Validation Failed").
		WithType(typeURI).
		WithInstance(c.Request.URL.Path).
		WithCause(err) // Chain the original error

	// Handle validation.Error specially
	if verr, ok := err.(*validation.Error); ok {
		p.WithDetail(fmt.Sprintf("Request validation failed with %d error(s)", len(verr.Fields)))
		p.WithExtension("errors", verr.Fields)

		if verr.Truncated {
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
func (c *Context) MustBindAndValidateRFC7807(out any, typeURI string, opts ...validation.Option) bool {
	if err := c.BindAndValidate(out, opts...); err != nil {
		_ = c.ValidationErrorRFC7807(err, typeURI)
		return false
	}
	return true
}

// BindAndValidateInto binds and validates into a specific type.
// This generic helper provides type-safe binding without needing to declare the variable.
//
// Example:
//
//	req, err := BindAndValidateInto[CreateUserRequest](c)
//	if err != nil {
//	    c.ValidationError(err, 400)
//	    return
//	}
//	// req is of type CreateUserRequest
func BindAndValidateInto[T any](c *Context, opts ...validation.Option) (T, error) {
	var out T
	if err := c.BindAndValidate(&out, opts...); err != nil {
		var zero T
		return zero, err
	}
	return out, nil
}

// MustBindAndValidateInto binds and validates, writing an error response on failure.
// Returns the typed value and a success flag.
// This method does NOT panic - it returns a boolean for control flow.
//
// Example:
//
//	req, ok := MustBindAndValidateInto[CreateUserRequest](c)
//	if !ok {
//	    return // Error already written
//	}
//	// req is of type CreateUserRequest
func MustBindAndValidateInto[T any](c *Context, opts ...validation.Option) (T, bool) {
	var out T
	if !c.MustBindAndValidate(&out, opts...) {
		var zero T
		return zero, false
	}
	return out, true
}
