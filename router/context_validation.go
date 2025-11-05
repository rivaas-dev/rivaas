package router

import "fmt"

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
