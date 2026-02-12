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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"rivaas.dev/binding"
	"rivaas.dev/router"
	"rivaas.dev/validation"

	riverrors "rivaas.dev/errors"
)

// Context wraps router.Context with app-level features including binding and validation.
// Context embeds router.Context to provide all HTTP routing functionality while adding
// high-level integration with binding and validation packages.
//
// Context instances are pooled by the App and reused across requests.
type Context struct {
	*router.Context // Embed router context for HTTP functionality

	app *App // Back reference to app for app-level services

	// Binding metadata (per-request)
	bindingMeta *bindingMetadata
}

// bindingMetadata holds a per-request binding state.
// bindingMetadata caches request body and tracks field presence for validation.
type bindingMetadata struct {
	bodyRead bool                   // Whether the body has been read
	rawBody  []byte                 // Cached raw body bytes
	presence validation.PresenceMap // Tracks which fields are present in the request
}

// Bind binds request data and validates it.
// Bind is the recommended method for handling request input.
//
// Bind automatically:
//   - Detects Content-Type and binds from appropriate sources
//   - Binds path, query, header, and cookie parameters based on struct tags
//   - Validates the bound struct using the configured strategy
//   - Tracks field presence for partial validation support
//
// Supported sources based on tags:
//   - path: "name"   - URL path parameters
//   - query: "name"  - Query string parameters
//   - header: "name" - HTTP headers
//   - cookie: "name" - Cookies
//   - json: "name"   - JSON request body
//   - form: "name"   - Form data (application/x-www-form-urlencoded or multipart/form-data)
//
// For binding without validation, use [Context.BindOnly].
// For separate binding and validation, use [Context.BindOnly] and [Context.Validate].
//
// Errors:
//   - [binding.ErrOutMustBePointer]: out is not a pointer to struct or map
//   - [binding.ErrRequestBodyNil]: request body is nil when JSON/form binding is needed
//   - [binding.ErrUnsupportedContentType]: Content-Type is not supported
//   - [validation.Error]: validation failed (one or more field errors)
//
// Example:
//
//	var req CreateUserRequest
//	if err := c.Bind(&req); err != nil {
//	    c.Fail(err)
//	    return
//	}
//
// With options:
//
//	if err := c.Bind(&req, app.WithStrict(), app.WithPartial()); err != nil {
//	    c.Fail(err)
//	    return
//	}
//
// Note: For multipart forms with file uploads, files must be retrieved
// separately using c.File() or c.Files().
func (c *Context) Bind(out any, opts ...BindOption) error {
	cfg := applyBindOptions(opts)

	// Step 1: Binding
	if err := c.bindInternal(out, cfg); err != nil {
		return err
	}

	// Step 2: Validation (unless skipped)
	if !cfg.skipValidation {
		if err := c.validateInternal(out, cfg); err != nil {
			return err
		}
	}

	return nil
}

// bindInternal performs the binding step with the given configuration.
func (c *Context) bindInternal(out any, cfg *bindConfig) error {
	// Build binding options
	bindOpts := cfg.bindingOpts
	if cfg.strict {
		bindOpts = append(bindOpts, binding.WithUnknownFields(binding.UnknownError))
	}
	// Get struct type for introspection
	t := reflect.TypeOf(out)
	if t.Kind() != reflect.Pointer {
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
			return c.bindJSON(out, bindOpts)
		case "application/x-www-form-urlencoded":
			return c.bindForm(out)
		case "multipart/form-data":
			return c.bindForm(out)
		default:
			// For maps, default to JSON even if the content-type is missing
			if isMap {
				return c.bindJSON(out, bindOpts)
			}

			return fmt.Errorf("%w: %s", binding.ErrUnsupportedContentType, contentType)
		}
	}

	return nil
}

// validateInternal performs the validation step with the given configuration.
func (c *Context) validateInternal(out any, cfg *bindConfig) error {
	ctx := c.Request.Context()

	// Inject raw JSON if available
	if c.bindingMeta != nil && len(c.bindingMeta.rawBody) > 0 {
		ctx = validation.InjectRawJSONCtx(ctx, c.bindingMeta.rawBody)
	}

	// Build validation options
	valOpts := []validation.Option{
		validation.WithContext(ctx),
	}

	// Handle partial validation
	if cfg.partial {
		valOpts = append(valOpts, validation.WithPartial(true))
	}

	// Inject presence map
	pm := cfg.presence
	if pm == nil {
		pm = c.Presence()
	}
	if pm != nil {
		valOpts = append(valOpts, validation.WithPresence(pm))
	}

	// Handle strict mode for validation
	if cfg.strict {
		valOpts = append(valOpts, validation.WithDisallowUnknownFields(true))
	}

	// Append user-provided validation options
	valOpts = append(valOpts, cfg.validationOpts...)

	return validation.Validate(ctx, out, valOpts...)
}

// hasJSONOrFormTag checks if the struct has any "json" or form tags.
func hasJSONOrFormTag(t reflect.Type) bool {
	return binding.HasStructTag(t, binding.TagJSON) || binding.HasStructTag(t, binding.TagForm)
}

// bindJSON binds JSON request body to a struct.
func (c *Context) bindJSON(out any, opts []binding.Option) error {
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

	// Translate binding.UnknownFieldError to validation.Error for consistency
	err := binding.JSONTo(c.bindingMeta.rawBody, out, opts...)
	var unkErr *binding.UnknownFieldError
	if errors.As(err, &unkErr) {
		return &validation.Error{
			Fields: []validation.FieldError{{
				Code:    "json.unknown_field",
				Message: unkErr.Error(),
			}},
		}
	}

	return err
}

// MustBind binds and validates, writing an error response on failure.
// Returns true if successful, false if an error was written.
//
// MustBind eliminates boilerplate error handling for the common case.
//
// Example:
//
//	var req CreateUserRequest
//	if !c.MustBind(&req) {
//	    return // Error already written
//	}
//	// Continue with validated request
func (c *Context) MustBind(out any, opts ...BindOption) bool {
	if err := c.Bind(out, opts...); err != nil {
		c.Fail(err)
		return false
	}
	return true
}

// BindOnly binds request data without validation.
// Use when you need fine-grained control over the bind/validate lifecycle.
//
// Example:
//
//	var req Request
//	if err := c.BindOnly(&req); err != nil {
//	    c.Fail(err)
//	    return
//	}
//	req.Normalize() // Custom processing
//	if err := c.Validate(&req); err != nil {
//	    c.Fail(err)
//	    return
//	}
func (c *Context) BindOnly(out any, opts ...BindOption) error {
	cfg := applyBindOptions(opts)
	return c.bindInternal(out, cfg)
}

// Validate validates a struct using the configured validation strategy.
// Use after [BindOnly] for fine-grained control.
//
// Example:
//
//	if err := c.Validate(&req, validation.WithPartial(true)); err != nil {
//	    c.Fail(err)
//	    return
//	}
func (c *Context) Validate(v any, opts ...validation.Option) error {
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

	return validation.Validate(ctx, v, allOpts...)
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

		return binding.MultipartTo(req.MultipartForm, out)
	}

	if err := req.ParseForm(); err != nil {
		return fmt.Errorf("failed to parse form: %w", err)
	}

	return binding.FormTo(req.Form, out)
}

// Fail responds with a formatted error using the configured formatter.
// Fail automatically aborts the handler chain after writing the response.
//
// Fail is the recommended way to return errors in handlers.
// The HTTP status is determined from the error (via ErrorType interface)
// or defaults to 500 Internal Server Error.
//
// Fail selects the formatter based on:
//   - Content negotiation (Accept header) if multiple formatters are configured
//   - Default formatter if single formatter is configured
//   - RFC 9457 formatter as ultimate fallback
//
// Example:
//
//	if err := c.Bind(&req); err != nil {
//	    c.Fail(err)
//	    return
//	}
//
//	if user == nil {
//	    c.Fail(fmt.Errorf("user not found"))
//	    return
//	}
//
// See also [Context.FailStatus] for explicit status codes and convenience methods
// like [Context.NotFound], [Context.BadRequest], [Context.Unauthorized].
//
// Note: This method shadows router.Context.Error() which collects errors without responding.
// Use c.Fail() to send an error response, or c.Context.Error() to collect errors.
func (c *Context) Fail(err error) {
	if err == nil {
		return
	}
	c.fail(err)
}

// FailStatus responds with an error and explicit status code.
// FailStatus automatically aborts the handler chain.
//
// FailStatus is useful when you want to override the error's default status.
//
// Example:
//
//	c.FailStatus(http.StatusNotFound, err)
//	c.FailStatus(http.StatusBadRequest, validationErr)
func (c *Context) FailStatus(status int, err error) {
	c.fail(riverrors.WithStatus(err, status))
}

// fail is the internal implementation that formats and writes the error response.
func (c *Context) fail(err error) {
	// Abort handler chain to prevent further processing
	c.Abort()

	// Select a formatter based on configuration
	formatter := c.selectFormatter()

	// Format the error (formatter is framework-agnostic)
	response := formatter.Format(c.Request, err)

	// Log error with request context (trace_id/span_id injected automatically by contextHandler)
	slog.ErrorContext(c.RequestContext(), "handler error",
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
		slog.ErrorContext(c.RequestContext(), "failed to write JSON response", "err", jsonErr)
	}
}

// selectFormatter chooses the appropriate formatter based on configuration.
// selectFormatter is a private helper used by Fail().
func (c *Context) selectFormatter() riverrors.Formatter {
	cfg := c.app.config.errors
	if cfg == nil {
		// Fallback to default
		return &riverrors.RFC9457{}
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

		// Predictable fallback - always use RFC9457
		return &riverrors.RFC9457{}
	}

	// Ultimate fallback
	return &riverrors.RFC9457{}
}

// NotFound responds with a 404 Not Found error.
// Pass nil for a generic "Not Found" message.
//
// Example:
//
//	c.NotFound(nil)                                    // generic "Not Found"
//	c.NotFound(fmt.Errorf("user %s not found", id))   // custom message
//	c.NotFound(ErrUserNotFound)                       // domain error
func (c *Context) NotFound(err error) {
	c.FailStatus(http.StatusNotFound, err)
}

// BadRequest responds with a 400 Bad Request error.
// Pass nil for a generic "Bad Request" message.
//
// Example:
//
//	c.BadRequest(nil)              // generic
//	c.BadRequest(validationErr)    // with validation details
func (c *Context) BadRequest(err error) {
	c.FailStatus(http.StatusBadRequest, err)
}

// Unauthorized responds with a 401 Unauthorized error.
// Pass nil for a generic "Unauthorized" message.
//
// Example:
//
//	c.Unauthorized(nil)                              // generic
//	c.Unauthorized(fmt.Errorf("invalid token"))     // custom message
func (c *Context) Unauthorized(err error) {
	c.FailStatus(http.StatusUnauthorized, err)
}

// Forbidden responds with a 403 Forbidden error.
// Pass nil for a generic "Forbidden" message.
//
// Example:
//
//	c.Forbidden(nil)                                      // generic
//	c.Forbidden(fmt.Errorf("insufficient permissions"))  // custom message
func (c *Context) Forbidden(err error) {
	c.FailStatus(http.StatusForbidden, err)
}

// Conflict responds with a 409 Conflict error.
// Pass nil for a generic "Conflict" message.
//
// Example:
//
//	c.Conflict(nil)                                    // generic
//	c.Conflict(fmt.Errorf("user already exists"))     // custom message
func (c *Context) Conflict(err error) {
	c.FailStatus(http.StatusConflict, err)
}

// Gone responds with a 410 Gone error.
// Pass nil for a generic "Gone" message.
//
// Example:
//
//	c.Gone(nil)                                        // generic
//	c.Gone(fmt.Errorf("resource permanently deleted")) // custom message
func (c *Context) Gone(err error) {
	c.FailStatus(http.StatusGone, err)
}

// UnprocessableEntity responds with a 422 Unprocessable Entity error.
// Pass nil for a generic "Unprocessable Entity" message.
//
// Example:
//
//	c.UnprocessableEntity(nil)           // generic
//	c.UnprocessableEntity(validationErr) // validation details
func (c *Context) UnprocessableEntity(err error) {
	c.FailStatus(http.StatusUnprocessableEntity, err)
}

// TooManyRequests responds with a 429 Too Many Requests error.
// Pass nil for a generic "Too Many Requests" message.
//
// Example:
//
//	c.TooManyRequests(nil)                                // generic
//	c.TooManyRequests(fmt.Errorf("rate limit exceeded")) // custom message
func (c *Context) TooManyRequests(err error) {
	c.FailStatus(http.StatusTooManyRequests, err)
}

// InternalError responds with a 500 Internal Server Error.
// Pass nil for a generic "Internal Server Error" message.
//
// Example:
//
//	c.InternalError(nil)   // generic
//	c.InternalError(err)   // with error details (logged but sanitized in response)
func (c *Context) InternalError(err error) {
	c.FailStatus(http.StatusInternalServerError, err)
}

// ServiceUnavailable responds with a 503 Service Unavailable error.
// Pass nil for a generic "Service Unavailable" message.
//
// Example:
//
//	c.ServiceUnavailable(nil)                              // generic
//	c.ServiceUnavailable(fmt.Errorf("maintenance mode"))  // custom message
func (c *Context) ServiceUnavailable(err error) {
	c.FailStatus(http.StatusServiceUnavailable, err)
}
