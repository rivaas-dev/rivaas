package router

// BindAndValidateInto binds and validates into a specific type.
// This generic helper provides type-safe binding without needing to declare the variable.
//
// Example:
//
//	req, err := c.BindAndValidateInto[CreateUserRequest]()
//	if err != nil {
//	    c.ValidationError(err, 400)
//	    return
//	}
//	// req is of type CreateUserRequest
func BindAndValidateInto[T any](c *Context, opts ...ValidationOption) (T, error) {
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
//	req, ok := c.MustBindAndValidateInto[CreateUserRequest]()
//	if !ok {
//	    return // Error already written
//	}
//	// req is of type CreateUserRequest
func MustBindAndValidateInto[T any](c *Context, opts ...ValidationOption) (T, bool) {
	var out T
	if !c.MustBindAndValidate(&out, opts...) {
		var zero T
		return zero, false
	}
	return out, true
}
