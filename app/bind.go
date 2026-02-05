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

package app

// Bind binds request data to type T and validates it.
// This is the recommended way to bind requests with type safety.
//
// Bind automatically:
//   - Detects Content-Type and binds from appropriate sources
//   - Binds path, query, header, and cookie parameters based on struct tags
//   - Validates the bound struct using the configured strategy
//   - Tracks field presence for partial validation support
//
// Errors:
//   - [binding.ErrOutMustBePointer]: T is not a struct type
//   - [binding.ErrUnsupportedContentType]: Content-Type not supported
//   - [validation.Error]: validation failed (one or more field errors)
//
// Example:
//
//	req, err := app.Bind[CreateUserRequest](c)
//	if err != nil {
//	    c.Fail(err)
//	    return
//	}
//	// req is of type CreateUserRequest
//
// With options:
//
//	req, err := app.Bind[CreateUserRequest](c, app.WithStrict())
func Bind[T any](c *Context, opts ...BindOption) (T, error) {
	var out T
	if err := c.Bind(&out, opts...); err != nil {
		var zero T
		return zero, err
	}
	return out, nil
}

// MustBind binds and validates, writing an error response on failure.
// Returns the bound value and true if successful.
//
// MustBind eliminates boilerplate error handling for the common case.
//
// Example:
//
//	req, ok := app.MustBind[CreateUserRequest](c)
//	if !ok {
//	    return // Error already written
//	}
//	// req is of type CreateUserRequest
func MustBind[T any](c *Context, opts ...BindOption) (T, bool) {
	var out T
	if !c.MustBind(&out, opts...) {
		var zero T
		return zero, false
	}
	return out, true
}

// BindOnly binds request data to type T without validation.
// Use when you need fine-grained control over the bind/validate lifecycle.
//
// Example:
//
//	req, err := app.BindOnly[Request](c)
//	if err != nil {
//	    c.Fail(err)
//	    return
//	}
//	req.Normalize() // Custom processing
//	if err := c.Validate(&req); err != nil {
//	    c.Fail(err)
//	    return
//	}
func BindOnly[T any](c *Context, opts ...BindOption) (T, error) {
	var out T
	if err := c.BindOnly(&out, opts...); err != nil {
		var zero T
		return zero, err
	}
	return out, nil
}

// BindPatch binds with partial validation enabled.
// Use for PATCH endpoints where only provided fields should be validated.
//
// Example:
//
//	req, err := app.BindPatch[UpdateUserRequest](c)
//
// Equivalent to:
//
//	req, err := app.Bind[UpdateUserRequest](c, app.WithPartial())
func BindPatch[T any](c *Context, opts ...BindOption) (T, error) {
	return Bind[T](c, append([]BindOption{WithPartial()}, opts...)...)
}

// MustBindPatch is the Must variant of [BindPatch].
// It binds with partial validation and writes an error response on failure.
//
// Example:
//
//	req, ok := app.MustBindPatch[UpdateUserRequest](c)
//	if !ok {
//	    return // Error already written
//	}
func MustBindPatch[T any](c *Context, opts ...BindOption) (T, bool) {
	return MustBind[T](c, append([]BindOption{WithPartial()}, opts...)...)
}

// BindStrict binds with unknown field rejection enabled.
// Use when you want to catch typos and API drift.
//
// Example:
//
//	req, err := app.BindStrict[CreateUserRequest](c)
//
// Equivalent to:
//
//	req, err := app.Bind[CreateUserRequest](c, app.WithStrict())
func BindStrict[T any](c *Context, opts ...BindOption) (T, error) {
	return Bind[T](c, append([]BindOption{WithStrict()}, opts...)...)
}

// MustBindStrict is the Must variant of [BindStrict].
// It binds with unknown field rejection and writes an error response on failure.
//
// Example:
//
//	req, ok := app.MustBindStrict[CreateUserRequest](c)
//	if !ok {
//	    return // Error already written
//	}
func MustBindStrict[T any](c *Context, opts ...BindOption) (T, bool) {
	return MustBind[T](c, append([]BindOption{WithStrict()}, opts...)...)
}
