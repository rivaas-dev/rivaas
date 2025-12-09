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

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"rivaas.dev/binding"
	"rivaas.dev/errors"
	"rivaas.dev/router"
	"rivaas.dev/validation"
)

// Context wraps router.Context with app-level features including binding and validation.
// Context embeds router.Context to provide all HTTP routing functionality while adding
// high-level integration with binding and validation packages.
//
// Context instances are pooled by the App and reused across requests.
type Context struct {
	*router.Context      // Embed router context for HTTP functionality
	app             *App // Back reference to app for app-level services

	// Binding metadata (per-request)
	bindingMeta *bindingMetadata

	// Request-scoped logger (never nil, includes HTTP metadata and trace correlation)
	logger *slog.Logger
}

// bindingMetadata holds per-request binding state.
// bindingMetadata caches request body and tracks field presence for validation.
type bindingMetadata struct {
	bodyRead bool                   // Whether the body has been read
	rawBody  []byte                 // Cached raw body bytes
	presence validation.PresenceMap // Tracks which fields are present in the request
}

// Bind automatically detects struct tags and binds from all relevant sources.
// Bind introspects the struct and binds values based on the tags present.
//
// Supported sources based on tags:
//   - path:"name"   - URL path parameters
//   - query:"name"  - Query string parameters
//   - header:"name" - HTTP headers
//   - cookie:"name" - Cookies
//   - json:"name"   - JSON request body
//   - form:"name"   - Form data (application/x-www-form-urlencoded or multipart/form-data)
//
// Bind only binds from sources where tags are present.
// For body binding (json/form), Bind automatically detects the Content-Type header.
// Bind defaults to JSON if Content-Type header is missing.
//
// Errors:
//   - [binding.ErrOutMustBePointer]: out is not a pointer to struct or map
//   - [binding.ErrRequestBodyNil]: request body is nil when JSON/form binding is needed
//   - [binding.ErrUnsupportedContentType]: Content-Type is not supported
//
// Example:
//
//	type GetUserRequest struct {
//	    ID      int    `path:"id"`
//	    Expand  string `query:"expand"`
//	    APIKey  string `header:"X-API-Key"`
//	    Session string `cookie:"session"`
//	}
//
//	var req GetUserRequest
//	if err := c.Bind(&req); err != nil {
//	    return err
//	}
//
// For binding + validation, use [Context.BindAndValidate].
//
// Note: For multipart forms with file uploads, files must be retrieved
// separately using c.File() or c.Files().
func (c *Context) Bind(out any) error {
	// Get struct type for introspection
	t := reflect.TypeOf(out)
	if t.Kind() != reflect.Ptr {
		return binding.ErrOutMustBePointer
	}
	if t.Elem().Kind() != reflect.Struct && t.Elem().Kind() != reflect.Map {
		return binding.ErrOutMustBePointer
	}

	elemType := t.Elem()
	isMap := elemType.Kind() == reflect.Map

	// For structs, bind from non-body sources (params, query, header, cookie)
	if !isMap {
		if err := binding.BindTo(out,
			binding.FromPath(c.AllParams()),
			binding.FromQuery(c.Request.URL.Query()),
			binding.FromHeader(c.Request.Header),
			binding.FromCookie(c.Request.Cookies()),
		); err != nil {
			return err
		}
	}

	// Handle body binding
	// For maps, always try to bind body (maps don't have struct tags)
	// For structs, check if they have json/form tags
	if isMap || hasJSONOrFormTag(elemType) {
		contentType := c.Request.Header.Get("Content-Type")

		// Extract base content type (remove parameters)
		if idx := strings.Index(contentType, ";"); idx != -1 {
			contentType = contentType[:idx]
		}
		contentType = strings.TrimSpace(strings.ToLower(contentType))

		switch contentType {
		case "application/json", "application/merge-patch+json", "application/json-patch+json", "":
			// Default to JSON if no content type specified
			return c.bindJSON(out)
		case "application/x-www-form-urlencoded":
			return c.bindForm(out)
		case "multipart/form-data":
			return c.bindForm(out)
		default:
			// For maps, default to JSON even if content-type is missing
			if isMap {
				return c.bindJSON(out)
			}

			return fmt.Errorf("%w: %s", binding.ErrUnsupportedContentType, contentType)
		}
	}

	return nil
}

// hasJSONOrFormTag checks if the struct has any json or form tags.
func hasJSONOrFormTag(t reflect.Type) bool {
	return binding.HasStructTag(t, binding.TagJSON) || binding.HasStructTag(t, binding.TagForm)
}

// bindJSON binds JSON request body to a struct.
func (c *Context) bindJSON(out any) error {
	req := c.Request
	if req.Body == nil {
		return binding.ErrRequestBodyNil
	}

	// Read and cache body
	if c.bindingMeta == nil {
		c.bindingMeta = &bindingMetadata{}
	}

	if !c.bindingMeta.bodyRead {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("failed to read body: %w", err)
		}
		c.bindingMeta.rawBody = body
		c.bindingMeta.bodyRead = true

		// Refill for downstream middleware
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		// Track presence using validation package
		if pm, presenceErr := validation.ComputePresence(body); presenceErr == nil {
			c.bindingMeta.presence = pm
		}

		// Store raw JSON in context for schema validation
		c.Request = c.Request.WithContext(
			validation.InjectRawJSONCtx(c.Request.Context(), body),
		)
	}

	return binding.JSONTo(c.bindingMeta.rawBody, out)
}

// BindJSONStrict binds JSON request body with unknown field rejection.
// BindJSONStrict is useful for catching typos and API drift early.
//
// Errors:
//   - [binding.ErrRequestBodyNil]: request body is nil
//   - [validation.Error]: JSON contains unknown fields (code: "json.unknown_field")
//
// Example:
//
//	var user User
//	if err := c.BindJSONStrict(&user); err != nil {
//	    // Returns error if JSON contains unknown fields
//	    return err
//	}
func (c *Context) BindJSONStrict(out any) error {
	req := c.Request
	if req.Body == nil {
		return binding.ErrRequestBodyNil
	}

	// Read and cache body (same as BindJSON)
	if c.bindingMeta == nil {
		c.bindingMeta = &bindingMetadata{}
	}

	if !c.bindingMeta.bodyRead {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("failed to read body: %w", err)
		}
		c.bindingMeta.rawBody = body
		c.bindingMeta.bodyRead = true

		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		// Track presence using validation package
		if pm, presenceErr := validation.ComputePresence(body); presenceErr == nil {
			c.bindingMeta.presence = pm
		}

		c.Request = c.Request.WithContext(
			validation.InjectRawJSONCtx(c.Request.Context(), body),
		)
	}

	// Use strict binding (reject unknown fields)
	err := binding.JSONTo(c.bindingMeta.rawBody, out, binding.WithUnknownFields(binding.UnknownError))

	// Translate binding.UnknownFieldError to validation.Error (only here!)
	if unkErr, ok := err.(*binding.UnknownFieldError); ok {
		return &validation.Error{
			Fields: []validation.FieldError{{
				Code:    "json.unknown_field",
				Message: unkErr.Error(),
			}},
		}
	}

	return err
}

// Presence returns the presence map for the current request.
// Presence returns nil if no binding has occurred yet.
//
// Example:
//
//	pm := c.Presence()
//	if pm != nil && pm.Has("email") {
//	    // email field was present in request
//	}
func (c *Context) Presence() validation.PresenceMap {
	if c.bindingMeta == nil {
		return nil
	}

	return c.bindingMeta.presence
}

// ResetBinding resets the binding metadata for this context.
// ResetBinding is useful for testing or when you need to rebind a request.
func (c *Context) ResetBinding() {
	c.bindingMeta = nil
}

// bindForm binds form data to a struct.
func (c *Context) bindForm(out any) error {
	req := c.Request
	contentType := req.Header.Get("Content-Type")

	// Parse the appropriate form type
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := req.ParseMultipartForm(32 << 20); err != nil { // 32 MB max
			return fmt.Errorf("failed to parse multipart form: %w", err)
		}
	} else {
		if err := req.ParseForm(); err != nil {
			return fmt.Errorf("failed to parse form: %w", err)
		}
	}

	return binding.FormTo(req.Form, out)
}

// BindAndValidate binds the request body and validates it.
// BindAndValidate is the most common validation pattern for handlers.
//
// BindAndValidate automatically:
//   - Injects the request context into validation options
//   - Injects the presence map for partial validation
//
// Errors:
//   - Binding errors from [Context.Bind] (wrapped with "binding failed:")
//   - [validation.Error]: one or more validation rules failed
//
// Example:
//
//	var req CreateUserRequest
//	if err := c.BindAndValidate(&req); err != nil {
//	    c.Error(err)
//	    return
//	}
//
// With options:
//
//	if err := c.BindAndValidate(&req,
//	    validation.WithStrategy(validation.StrategyTags),
//	    validation.WithPartial(true),
//	); err != nil {
//	    c.Error(err)
//	    return
//	}
func (c *Context) BindAndValidate(out any, opts ...validation.Option) error {
	if err := c.Bind(out); err != nil {
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
// BindAndValidateStrict is useful for catching typos and API drift early.
//
// Errors:
//   - [validation.Error]: JSON contains unknown fields or validation failed
//   - [binding.ErrRequestBodyNil]: request body is nil
//
// Example:
//
//	var req CreateUserRequest
//	if err := c.BindAndValidateStrict(&req); err != nil {
//	    c.Error(err)
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

// Error responds with a formatted error using the configured formatter.
// Error is the recommended way to return errors in handlers.
//
// Error selects the formatter based on:
//   - Content negotiation (Accept header) if multiple formatters are configured
//   - Default formatter if single formatter is configured
//   - RFC 9457 formatter as ultimate fallback
//
// Example:
//
//	if err := c.BindAndValidate(&req); err != nil {
//	    c.Error(err)
//	    return
//	}
//
//	if user == nil {
//	    c.Error(fmt.Errorf("user not found"))
//	    return
//	}
//
// See also [Context.ErrorStatus] for explicit status codes and convenience methods
// like [Context.NotFound], [Context.BadRequest], [Context.Unauthorized].
func (c *Context) Error(err error) {
	if err == nil {
		return
	}

	// Select formatter based on configuration
	formatter := c.selectFormatter()

	// Format the error (formatter is framework-agnostic)
	response := formatter.Format(c.Request, err)

	// Log error using request-scoped logger (includes trace context, request ID, etc.)
	// Logger() is always safe to call - uses noopLogger if logging isn't configured
	c.Logger().Error("handler error",
		"error", err,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"status", response.Status,
	)

	// Write response
	c.Header("Content-Type", response.ContentType)
	if response.Headers != nil {
		for key, values := range response.Headers {
			for _, value := range values {
				c.Header(key, value)
			}
		}
	}

	if jsonErr := c.JSON(response.Status, response.Body); jsonErr != nil {
		c.Logger().Error("failed to write JSON response", "err", jsonErr)
	}
}

// selectFormatter chooses the appropriate formatter based on configuration.
// selectFormatter is a private helper used by Error().
func (c *Context) selectFormatter() errors.Formatter {
	cfg := c.app.config.errors
	if cfg == nil {
		// Fallback to default
		return &errors.RFC9457{}
	}

	// Single formatter mode
	if cfg.formatter != nil {
		return cfg.formatter
	}

	// Multi-formatter mode with content negotiation
	if len(cfg.formatters) > 0 {
		// Build list of acceptable media types
		offers := make([]string, 0, len(cfg.formatters))
		for mediaType := range cfg.formatters {
			offers = append(offers, mediaType)
		}

		// Use router's content negotiation
		match := c.Accepts(offers...)
		if match != "" {
			if formatter, ok := cfg.formatters[match]; ok {
				return formatter
			}
		}

		// Fallback to default format
		if cfg.defaultFormat != "" {
			if formatter, ok := cfg.formatters[cfg.defaultFormat]; ok {
				return formatter
			}
		}

		// Last resort: use first formatter
		for _, formatter := range cfg.formatters {
			return formatter
		}
	}

	// Ultimate fallback
	return &errors.RFC9457{}
}

// ErrorStatus responds with an error and explicit status code.
// ErrorStatus is useful when you want to override the error's default status.
//
// Example:
//
//	c.ErrorStatus(err, 404)
func (c *Context) ErrorStatus(err error, status int) {
	// Wrap error to override status
	c.Error(&statusError{err: err, status: status})
}

// statusError wraps an error with an explicit status code.
type statusError struct {
	err    error
	status int
}

func (e *statusError) Error() string {
	return e.err.Error()
}

func (e *statusError) Unwrap() error {
	return e.err
}

func (e *statusError) HTTPStatus() int {
	return e.status
}

// NotFound responds with a 404 Not Found error.
// NotFound is a convenience method for 404 errors.
//
// Example:
//
//	if user == nil {
//	    c.NotFound("user not found")
//	    return
//	}
func (c *Context) NotFound(message string) {
	c.ErrorStatus(fmt.Errorf("%s", message), http.StatusNotFound)
}

// BadRequest responds with a 400 Bad Request error.
// BadRequest is a convenience method for 400 errors.
//
// Example:
//
//	if err := validateInput(input); err != nil {
//	    c.BadRequest("invalid input")
//	    return
//	}
func (c *Context) BadRequest(message string) {
	c.ErrorStatus(fmt.Errorf("%s", message), http.StatusBadRequest)
}

// Unauthorized responds with a 401 Unauthorized error.
// Unauthorized is a convenience method for 401 errors.
//
// Example:
//
//	if !isAuthenticated {
//	    c.Unauthorized("authentication required")
//	    return
//	}
func (c *Context) Unauthorized(message string) {
	c.ErrorStatus(fmt.Errorf("%s", message), http.StatusUnauthorized)
}

// Forbidden responds with a 403 Forbidden error.
// Forbidden is a convenience method for 403 errors.
//
// Example:
//
//	if !hasPermission {
//	    c.Forbidden("insufficient permissions")
//	    return
//	}
func (c *Context) Forbidden(message string) {
	c.ErrorStatus(fmt.Errorf("%s", message), http.StatusForbidden)
}

// InternalError responds with a 500 Internal Server Error.
// InternalError is a convenience method for 500 errors.
//
// Example:
//
//	if err := processRequest(); err != nil {
//	    c.InternalError(err)
//	    return
//	}
func (c *Context) InternalError(err error) {
	c.ErrorStatus(err, http.StatusInternalServerError)
}

// MustBindAndValidate binds and validates, writing an error response on failure.
// MustBindAndValidate returns true if validation succeeded, false otherwise.
// MustBindAndValidate does not panic - it returns a boolean for control flow.
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
		c.Error(err)
		return false
	}

	return true
}

// BindAndValidateInto binds and validates into a specific type.
// T must be a struct type with exported fields; using non-struct types
// returns [binding.ErrOutMustBePointer].
//
// BindAndValidateInto is a generic helper that provides type-safe binding
// without needing to declare the variable separately.
//
// Example:
//
//	req, err := BindAndValidateInto[CreateUserRequest](c)
//	if err != nil {
//	    c.Error(err)
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
// T must be a struct type with exported fields; using non-struct types
// writes an error response and returns false.
//
// MustBindAndValidateInto returns the typed value and a success flag.
// It does not panic - it returns a boolean for control flow.
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

// Logger returns the request-scoped logger.
// Logger returns a logger that is automatically configured with:
//   - HTTP metadata using semantic conventions (method, route, target, client IP)
//   - Request ID (if present in X-Request-ID header)
//   - Trace/span IDs (if OpenTelemetry tracing is enabled)
//   - Service metadata (from base logger configuration)
//
// Logger never returns nil. If no logger is configured at the app level,
// a no-op logger is returned (logs are silently discarded).
//
// Field naming follows OpenTelemetry semantic conventions for consistency
// with metrics and traces.
//
// Example:
//
//	app.GET("/orders/:id", func(c *app.Context) {
//	    c.Logger().Info("processing order",
//	        slog.String("order.id", c.Param("id")),
//	        slog.Int("customer.id", customerID),
//	    )
//	})
//
// Log output includes automatic context:
//
//	{
//	  "time": "2024-...",
//	  "level": "INFO",
//	  "msg": "processing order",
//	  "http.method": "GET",
//	  "http.route": "/orders/:id",
//	  "http.target": "/orders/123",
//	  "network.client.ip": "203.0.113.1",
//	  "trace_id": "abc...",       // if tracing enabled
//	  "span_id": "def...",        // if tracing enabled
//	  "order.id": "123"
//	}
func (c *Context) Logger() *slog.Logger {
	return c.logger
}
