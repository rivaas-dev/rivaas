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

package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

// Context represents the context of the current HTTP request.
// It provides access to request/response objects, URL parameters,
// and middleware chain execution.
//
// The Context includes several features:
//   - Parameter storage and lookup
//   - Response methods
//   - Middleware chain execution
//   - OpenTelemetry tracing support (when enabled)
//   - Custom metrics recording capabilities
//   - Route versioning support for API versioning
//
// ⚠️ THREAD SAFETY: Context is NOT thread-safe.
// A Context instance is bound to a single HTTP request and must only be accessed
// by the goroutine handling that request. Do not pass Context to other goroutines
// or access it concurrently from multiple goroutines.
//
// ⚠️ MEMORY SAFETY: Context objects are pooled and reused.
//
// CRITICAL RULES:
//  1. DO NOT retain references to Context objects beyond the request handler lifetime.
//  2. The router automatically returns contexts to the pool after request completion.
//  3. DO NOT access Context concurrently from multiple goroutines - it is NOT thread-safe.
//  4. For async operations, copy needed data from Context before starting goroutines.
//
// Why this matters:
//   - Contexts are reused across requests
//   - Retaining references causes memory leaks and data corruption
//   - Concurrent access causes data races
//
// Example (CORRECT - context used within handler):
//
//	func handler(c *router.Context) {
//	    userID := c.Param("id")
//	    c.JSON(http.StatusOK, map[string]string{"id": userID})
//	    // Context automatically returned to pool by router
//	}
//
// Example (CORRECT - async operation with copied data):
//
//	func handler(c *router.Context) {
//	    // Copy needed data before starting goroutine
//	    userID := c.Param("id")
//	    go func(id string) {
//	        // Process async work with copied data...
//	    }(userID)
//	}
//
// Example (WRONG - retaining context reference):
//
//	var globalContext *router.Context // BAD!
//
//	func handler(c *router.Context) {
//	    globalContext = c // BAD! Memory leak and data corruption
//	}
//
// Memory Layout:
//
// Fields are organized with commonly accessed fields grouped together.
//
// Layout details:
//   - Commonly accessed fields (Request, Response, handlers, router) are grouped together
//   - Parameter arrays are grouped together
//   - Less commonly accessed fields (Params map, logger, etc.) are placed at the end
//
// NOTE: Go compiler controls actual field ordering. The compiler may reorder fields.
// Use `go tool compile -S` or `unsafe.Offsetof()` to verify layout on your specific architecture.
//
// Parameter Storage Strategy:
//
// Context uses a hybrid parameter storage approach:
//
// For routes with a limited number of parameters:
//   - Uses fixed-size arrays (paramKeys[8], paramValues[8])
//   - Params map remains nil
//
// For routes with many parameters:
//   - First parameters stored in arrays
//   - Remaining params overflow to Params map
//   - Param() checks arrays first, then falls back to map
//   - Gracefully handles edge cases without complexity
//
// If you have routes with many parameters, consider refactoring your API design.
type Context struct {
	// Core request fields - accessed on every HTTP request.
	Request  *http.Request       // The HTTP request object
	Response http.ResponseWriter // The HTTP response writer
	handlers []HandlerFunc       // Handler chain for this request
	router   *Router             // Reference to the router for metrics access

	index      int32 // Current handler index in the chain
	paramCount int32 // Number of parameters in arrays (0-8)

	// Parameter storage - arrays provide storage for route parameters.
	paramKeys   [8]string // Parameter names
	paramValues [8]string // Parameter values

	// Additional fields - accessed in specific scenarios.
	//
	// Params map is nil unless route has many parameters.
	// When Params is populated, it contains additional parameters while paramKeys/paramValues
	// hold the initial parameters.
	Params          map[string]string      // URL parameters (nil when not needed, populated when needed)
	span            trace.Span             // Current OpenTelemetry span
	metricsRecorder ContextMetricsRecorder // Metrics recorder for this context
	tracingRecorder ContextTracingRecorder // Tracing recorder for this context
	version         string                 // Current API version (e.g., "v1", "v2")
	routePattern    string                 // Matched route pattern (e.g., "/users/:id" or "_not_found")
	logger          *slog.Logger           // Request-scoped logger (set by observability recorder)

	// Header parsing cache (per-request)
	cachedAcceptHeader string       // Cached Accept header value
	cachedAcceptSpecs  []acceptSpec // Parsed Accept header specs
	cachedArena        *headerArena // Arena allocator for spec buffers (pooled)

	// Abort flag to stop handler chain execution
	aborted bool // Set to true when Abort() is called

	// Error collection: Slice of errors collected during request processing.
	// Errors are collected via Error() method and can be processed later.
	errors []error // Lazy initialization - only created when Error() is called
}

// HandlerFunc defines the handler function signature for route handlers and middleware.
// Handlers receive a Context object containing request information and response writer.
//
// Example handler:
//
//	func MyHandler(c *router.Context) {
//	    userID := c.Param("id")
//	    c.JSON(http.StatusOK, map[string]string{"user_id": userID})
//	}
//
// Example middleware:
//
//	func Logger() router.HandlerFunc {
//	    return func(c *router.Context) {
//	        start := time.Now()
//	        c.Next() // Execute next handler
//	        log.Printf("Request took %v", time.Since(start))
//	    }
//	}
type HandlerFunc func(*Context)

// NewContext creates a new context instance for the given HTTP request and response.
// This function is primarily used internally by the router, but can be useful for testing.
//
// Note: In normal operation, contexts are obtained from a pool.
// Only use this function when you need to create a context outside the normal request flow.
func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		Request:  r,
		Response: w,
		index:    -1,
		router:   nil, // Will be set when needed for metrics
		// Simple initialization
	}
}

// NewPooledContext creates a new context for use in pools.
// This is used by the pool package to create contexts with the router reference set.
// If preallocateMap is true, the Params map is pre-allocated for routes with >8 parameters.
func NewPooledContext(r *Router, preallocateMap bool) *Context {
	ctx := &Context{
		router:      r,
		paramKeys:   [8]string{},
		paramValues: [8]string{},
		index:       -1,
		paramCount:  0,
	}
	if preallocateMap {
		ctx.Params = make(map[string]string, 16)
	}
	ctx.reset()

	return ctx
}

// Next executes the next handler in the middleware chain.
// This method should be called by middleware to continue execution.
// If Next() is not called, the remaining handlers in the chain will not execute.
//
// Example middleware usage:
//
//	func AuthMiddleware() router.HandlerFunc {
//	    return func(c *router.Context) {
//	        if !isAuthenticated(c.Request) {
//	            c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
//	            c.Abort() // Stop the chain
//	            return
//	        }
//	        c.Next() // Continue to next handler
//	    }
//	}
func (c *Context) Next() {
	c.index++
	handlersLen := int32(len(c.handlers))

	// Pre-compute length, check cancellation only if enabled
	if c.router != nil && c.router.checkCancellation {
		// With cancellation checks (default behavior)
		for c.index < handlersLen {
			// Check if handler chain was aborted
			if c.aborted {
				return
			}
			// Check for context cancellation to avoid processing canceled requests
			// This is important for long-running handler chains or I/O operations
			if err := c.Request.Context().Err(); err != nil {
				return // Context canceled or deadline exceeded
			}
			c.handlers[c.index](c)
			c.index++
		}
	} else {
		// Path without cancellation checks
		for c.index < handlersLen {
			// Check if handler chain was aborted
			if c.aborted {
				return
			}
			c.handlers[c.index](c)
			c.index++
		}
	}
}

// Abort stops the handler chain from executing any further handlers.
// This is useful for middleware that needs to prevent further processing,
// such as authentication failures or request validation errors.
//
// Note: Handlers that have already been executed will not be affected.
// Only handlers later in the chain will be skipped.
//
// Example:
//
//	func AuthMiddleware() router.HandlerFunc {
//	    return func(c *router.Context) {
//	        if !isAuthenticated(c.Request) {
//	            c.JSON(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
//	            c.Abort()
//	            return
//	        }
//	        c.Next()
//	    }
//	}
func (c *Context) Abort() {
	c.aborted = true
}

// IsAborted returns true if the handler chain has been aborted.
// This allows handlers to check if they should continue processing.
func (c *Context) IsAborted() bool {
	return c.aborted
}

// Param returns the value of the URL parameter by key.
// It extracts parameters from the matched route path.
//
// For routes with parameters like "/users/:id/posts/:post_id", extract values:
//
//	userID := c.Param("id")
//	postID := c.Param("post_id")
//
// Returns an empty string if the parameter doesn't exist.
//
// Example:
//
//	r.GET("/users/:id", func(c *router.Context) {
//	    userID := c.Param("id")
//	    c.JSON(http.StatusOK, map[string]string{"user_id": userID})
//	})
//
//go:inline
func (c *Context) Param(key string) string {
	// Array lookup first
	for i := range c.paramCount {
		if c.paramKeys[i] == key {
			return c.paramValues[i]
		}
	}
	// Fallback to map for >8 parameters (rare case)
	return c.Params[key]
}

// JSON sends a JSON response with the specified status code.
// Returns an error if encoding or writing fails.
//
// Example:
//
//	if err := c.JSON(http.StatusOK, user); err != nil {
//	    c.Logger().Error("failed to write json", "err", err)
//	    return
//	}
func (c *Context) JSON(code int, obj any) error {
	// Encode to buffer first to catch errors before writing headers
	// This prevents inconsistent response state if encoding fails
	var buf strings.Builder
	buf.Grow(256) // Pre-allocate reasonable size for most JSON responses

	if err := json.NewEncoder(&buf).Encode(obj); err != nil {
		// Return error without writing anything - caller can handle it
		return fmt.Errorf("JSON encoding failed for type %T: %w", obj, err)
	}

	// Only write headers after successful encoding
	if c.Response == nil {
		return ErrContextResponseNil
	}
	c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Check if headers have already been written to avoid "superfluous response.WriteHeader call"
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	// Write the pre-encoded JSON
	_, writeErr := c.Response.Write([]byte(buf.String()))

	return writeErr
}

// IndentedJSON sends a JSON response with indentation for readability.
// Returns an error if encoding or writing fails.
//
// This is useful for debugging, development, and human-readable API responses.
// Use JSON() for compact responses, IndentedJSON() for debugging/development.
func (c *Context) IndentedJSON(code int, obj any) error {
	// Use MarshalIndent for pretty-printing
	jsonBytes, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("IndentedJSON encoding failed for type %T: %w", obj, err)
	}

	// Set headers
	c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Write status code
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	// Write the formatted JSON
	_, writeErr := c.Response.Write(jsonBytes)

	return writeErr
}

// PureJSON sends a JSON response without escaping HTML characters.
// Returns an error if encoding or writing fails.
//
// Unlike JSON(), this does not escape <, >, &, and other HTML characters.
// Use cases: HTML/markdown content, URLs with query parameters, code snippets.
func (c *Context) PureJSON(code int, obj any) error {
	// Encode without HTML escaping
	var buf strings.Builder
	buf.Grow(256)

	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // Don't escape <, >, &

	if err := encoder.Encode(obj); err != nil {
		return fmt.Errorf("PureJSON encoding failed for type %T: %w", obj, err)
	}

	// Set headers
	c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Write status code
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	// Write the JSON
	_, writeErr := c.Response.Write([]byte(buf.String()))

	return writeErr
}

// SecureJSON sends a JSON response with a security prefix to prevent JSON hijacking.
// Returns an error if encoding or writing fails.
//
// Default prefix: "while(1);" (matches Gin's default).
// The client must strip this prefix before parsing JSON.
func (c *Context) SecureJSON(code int, obj any, prefix ...string) error {
	// Determine security prefix
	securityPrefix := "while(1);"
	if len(prefix) > 0 && prefix[0] != "" {
		securityPrefix = prefix[0]
	}

	// Encode JSON
	var buf strings.Builder
	buf.Grow(256 + len(securityPrefix))

	if err := json.NewEncoder(&buf).Encode(obj); err != nil {
		return fmt.Errorf("SecureJSON encoding failed for type %T: %w", obj, err)
	}

	// Set headers
	c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Write status code
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	// Write security prefix + JSON
	// Note: json.Encoder.Encode() adds a newline, we keep it for compatibility
	response := securityPrefix + buf.String()
	_, writeErr := c.Response.Write([]byte(response))

	return writeErr
}

// ASCIIJSON sends a JSON response with all non-ASCII characters escaped to \uXXXX.
// Returns an error if encoding or writing fails.
//
// This ensures the response is pure ASCII, useful for legacy systems or strict compatibility.
// All non-ASCII characters are escaped to their Unicode code point representation (\uXXXX).
func (c *Context) ASCIIJSON(code int, obj any) error {
	// Use json.Marshal which already escapes non-ASCII to \uXXXX by default
	// when using the default encoder settings
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false) // Don't escape HTML, but still escape Unicode

	if err := encoder.Encode(obj); err != nil {
		return fmt.Errorf("ASCIIJSON encoding failed for type %T: %w", obj, err)
	}

	// Get the JSON bytes
	jsonBytes := buf.Bytes()

	// Additional pass to ensure ALL non-ASCII bytes are escaped
	// This handles edge cases where json.Encoder might miss some
	var result strings.Builder
	result.Grow(len(jsonBytes) * 2)

	i := 0
	for i < len(jsonBytes) {
		b := jsonBytes[i]
		if b >= 128 {
			// Multi-byte UTF-8 sequence - decode and escape as Unicode
			r, size := decodeRuneInJSON(jsonBytes[i:])
			if size > 0 {
				// Escape the full rune
				if r <= 0xFFFF {
					result.WriteString(fmt.Sprintf("\\u%04x", r))
				} else {
					// Surrogate pair for runes > U+FFFF
					r -= 0x10000
					result.WriteString(fmt.Sprintf("\\u%04x\\u%04x", 0xD800+(r>>10), 0xDC00+(r&0x3FF)))
				}
				i += size

				continue
			}
			// Fallback: escape single byte
			result.WriteString(fmt.Sprintf("\\u%04x", b))
			i++
		} else {
			// ASCII character - write as-is
			result.WriteByte(b)
			i++
		}
	}

	// Set headers
	c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Write status code
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	// Write the ASCII-safe JSON
	_, writeErr := c.Response.Write([]byte(result.String()))

	return writeErr
}

// decodeRuneInJSON decodes a UTF-8 rune from a byte slice (inside JSON context)
// Returns the rune and the number of bytes consumed
func decodeRuneInJSON(b []byte) (rune, int) {
	if len(b) == 0 {
		return 0, 0
	}

	// Single byte (ASCII)
	if b[0] < 128 {
		return rune(b[0]), 1
	}

	// Multi-byte UTF-8
	if len(b) >= 2 && (b[0]&0xE0) == 0xC0 {
		// 2-byte sequence
		return rune((b[0]&0x1F))<<6 | rune(b[1]&0x3F), 2
	}
	if len(b) >= 3 && (b[0]&0xF0) == 0xE0 {
		// 3-byte sequence
		return rune((b[0]&0x0F))<<12 | rune((b[1]&0x3F))<<6 | rune(b[2]&0x3F), 3
	}
	if len(b) >= 4 && (b[0]&0xF8) == 0xF0 {
		// 4-byte sequence
		return rune((b[0]&0x07))<<18 | rune((b[1]&0x3F))<<12 | rune((b[2]&0x3F))<<6 | rune(b[3]&0x3F), 4
	}

	return 0, 0
}

// String sends a plain text response.
// This method does NOT perform formatting - the value is used as-is.
// For formatting with values, use Stringf.
// Returns an error if writing fails.
//
// Note: This method converts string to []byte
// in the underlying Write() call. This is unavoidable when writing strings
// through Go's standard http.ResponseWriter interface.
//
// Example:
//
//	if err := c.String(200, "Hello, World!"); err != nil {
//	    return err
//	}
//
// For static responses, use a pre-allocated []byte:
//
//	staticBytes := []byte("Hello, World!")
//	c.Response.WriteHeader(200)
//	c.Response.Write(staticBytes)
func (c *Context) String(code int, value string) error {
	if c.Response.Header().Get("Content-Type") == "" {
		c.Response.Header().Set("Content-Type", "text/plain")
	}

	// Check if headers have already been written
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	// String-to-[]byte conversion
	_, err := c.Response.Write([]byte(value))
	if err != nil {
		return fmt.Errorf("writing string response: %w", err)
	}

	return nil
}

// Stringf sends a formatted plain text response.
// This uses fmt.Sprintf-style formatting and supports any number of values.
// Returns an error if writing fails.
//
// Note: This method uses variadic parameters
// (values ...any). For plain strings without formatting, use String() instead,
// which avoids variadic slice creation but still converts
// string-to-[]byte conversion.
//
// Example:
//
//	if err := c.Stringf(200, "User: %s", userName); err != nil {
//	    return err
//	}
func (c *Context) Stringf(code int, format string, values ...any) error {
	if c.Response.Header().Get("Content-Type") == "" {
		c.Response.Header().Set("Content-Type", "text/plain")
	}

	// Check if headers have already been written
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	// Handle single %s pattern with single string value (fast path)
	if err := c.tryFastStringFormat(format, values); err == nil {
		return nil
	}

	// Fallback for complex formatting (multiple values, non-string types, etc.)
	// Direct fmt.Fprintf to response
	_, err := fmt.Fprintf(c.Response, format, values...)
	if err != nil {
		return fmt.Errorf("writing formatted string response: %w", err)
	}

	return nil
}

// errFastPathNotApplicable is a sentinel error for fast path optimization.
var errFastPathNotApplicable = errors.New("fast path not applicable")

// tryFastStringFormat attempts to write a single %s format without fmt.Sprintf allocation.
// Returns nil on success, errFastPathNotApplicable if fast path doesn't apply.
func (c *Context) tryFastStringFormat(format string, values []any) error {
	// Only applies when: 1 value, value is string, exactly 1 %s in format
	if len(values) != 1 {
		return errFastPathNotApplicable
	}

	v, ok := values[0].(string)
	if !ok || strings.Count(format, "%s") != 1 {
		return errFastPathNotApplicable
	}

	idx := strings.Index(format, "%s")
	if idx == -1 {
		return errFastPathNotApplicable
	}

	// Write directly to response in chunks
	// Use unsafe zero-copy string->bytes conversion (read-only, safe in this context)
	if idx > 0 {
		if _, err := c.Response.Write(unsafeStringToBytes(format[:idx])); err != nil {
			return fmt.Errorf("writing formatted string response: %w", err)
		}
	}
	if len(v) > 0 {
		if _, err := c.Response.Write(unsafeStringToBytes(v)); err != nil {
			return fmt.Errorf("writing formatted string response: %w", err)
		}
	}
	if idx+2 < len(format) {
		if _, err := c.Response.Write(unsafeStringToBytes(format[idx+2:])); err != nil {
			return fmt.Errorf("writing formatted string response: %w", err)
		}
	}

	return nil
}

// HTML sends an HTML response with the specified status code.
// Returns an error if writing fails.
func (c *Context) HTML(code int, html string) error {
	c.Response.Header().Set("Content-Type", "text/html")

	// Check if headers have already been written to avoid "superfluous response.WriteHeader call"
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	_, err := c.Response.Write([]byte(html))
	if err != nil {
		return fmt.Errorf("writing HTML response: %w", err)
	}

	return nil
}

// Status sets the HTTP status code for the response.
// This should be called before writing any response body.
//
// Example:
//
//	c.Status(http.StatusNoContent) // 204 No Content
func (c *Context) Status(code int) {
	// Check if headers have already been written to avoid "superfluous response.WriteHeader call"
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}
}

// Header sets a response header with automatic security sanitization.
// Headers must be set before writing the response body.
//
// SECURITY: This method automatically sanitizes header values to prevent header injection attacks.
// Header values containing newline characters (\r or \n) are automatically stripped and logged.
//
// BREAKING CHANGE: Previously this method would panic on invalid headers. Now it sanitizes
// them and logs a warning. This is safer for production but may hide bugs during development.
// Use the logger to catch these issues in development/testing.
//
// Example:
//
//	c.Header("Cache-Control", "no-cache")
//	c.Header("Content-Type", "application/pdf")
//	c.Header("X-User-Agent", userAgent) // Automatically sanitized if contains newlines
func (c *Context) Header(key, value string) {
	// Detect and sanitize header injection attempts
	if strings.ContainsAny(value, "\r\n") {
		// Report security event if diagnostics handler is configured
		if c.router != nil {
			c.router.emit(DiagHeaderInjection, "header injection attempt blocked and sanitized", map[string]any{
				"key":            key,
				"original_value": value,
				"path":           c.Request.URL.Path,
				"client_ip":      c.ClientIP(),
				"user_agent":     c.Request.UserAgent(),
			})
		}

		// Sanitize by removing newline characters
		value = strings.ReplaceAll(value, "\r", "")
		value = strings.ReplaceAll(value, "\n", "")
	}

	c.Response.Header().Set(key, value)
}

// Query returns the value of the URL query parameter by key.
// Returns an empty string if the parameter doesn't exist.
//
// For a URL like "/search?q=golang&limit=10":
//
//	query := c.Query("q")     // "golang"
//	limit := c.Query("limit") // "10"
//	missing := c.Query("xyz") // ""
func (c *Context) Query(key string) string {
	if c.Request == nil {
		return ""
	}

	return c.Request.URL.Query().Get(key)
}

// FormValue returns the value of the form parameter from POST request body.
// This works for both application/x-www-form-urlencoded and multipart/form-data.
// Returns an empty string if the parameter doesn't exist.
//
// Example:
//
//	username := c.FormValue("username")
//	password := c.FormValue("password")
func (c *Context) FormValue(key string) string {
	return c.Request.FormValue(key)
}

// Redirect sends an HTTP redirect response with the specified status code and location.
// Common status codes: 301 (Moved Permanently), 302 (Found), 303 (See Other), 307 (Temporary Redirect)
//
// Example:
//
//	c.Redirect(http.StatusFound, "/login")
//	c.Redirect(http.StatusMovedPermanently, "https://newdomain.com")
func (c *Context) Redirect(code int, location string) {
	c.Header("Location", location)
	c.Status(code)
}

// ServeFile serves a file from the filesystem to the client.
// This handles proper content types, range requests, and caching headers.
//
// Example:
//
//	c.ServeFile("./uploads/document.pdf")
//	c.ServeFile("/var/www/static/image.jpg")
func (c *Context) ServeFile(filepath string) {
	http.ServeFile(c.Response, c.Request, filepath)
}

// NoContent sends a 204 No Content response.
// This is a convenience method for APIs that don't return data.
func (c *Context) NoContent() {
	c.Status(http.StatusNoContent)
}

// QueryDefault returns the query parameter value or a default if not present.
// This avoids the need for manual empty string checking.
//
// Example:
//
//	limit := c.QueryDefault("limit", "10")
//	page := c.QueryDefault("page", "1")
func (c *Context) QueryDefault(key, defaultValue string) string {
	value := c.Query(key)
	if value == "" {
		return defaultValue
	}

	return value
}

// FormValueDefault returns the form parameter value or a default if not present.
// This avoids the need for manual empty string checking.
//
// Example:
//
//	username := c.FormValueDefault("username", "guest")
func (c *Context) FormValueDefault(key, defaultValue string) string {
	value := c.FormValue(key)
	if value == "" {
		return defaultValue
	}

	return value
}

// Version returns the current API version for this request.
// Returns an empty string if versioning is not enabled or no version is detected.
//
// Example:
//
//	version := c.Version() // "v1", "v2", etc.
//	if c.IsVersion("v2") {
//	    // Handle v2 specific logic
//	}
func (c *Context) Version() string {
	// Return the version set during routing.
	// For versioned routes (r.Version().GET), this is set to the detected version.
	// For non-versioned routes (r.GET), this is empty string.
	// No lazy detection - version is determined at route time, not handler time.
	return c.version
}

// Logger returns the request-scoped logger for this context.
// The logger is set by the observability recorder during request initialization
// and includes request metadata like method, path, trace ID, etc.
//
// Returns a non-nil logger. If no logger was set, returns a no-op logger.
func (c *Context) Logger() *slog.Logger {
	if c.logger != nil {
		return c.logger
	}

	return noopLogger
}

// IsVersion returns true if the current request is for the specified version.
// This is a helper for version checking.
//
// Example:
//
//	if c.IsVersion("v1") {
//	    // Handle v1 specific logic
//	} else if c.IsVersion("v2") {
//	    // Handle v2 specific logic
//	}
func (c *Context) IsVersion(version string) bool {
	return c.version == version
}

// SetCookie sets a cookie with the given name and value.
// This is a convenience wrapper around http.SetCookie.
//
// Example:
//
//	c.SetCookie("session_id", "abc123", 3600, "/", "", false, true)
func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
	}
	http.SetCookie(c.Response, cookie)
}

// GetCookie returns the value of the named cookie or an error if not found.
// The value is automatically URL-unescaped.
func (c *Context) GetCookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	value, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return "", err
	}

	return value, nil
}

// YAML sends a YAML response with the specified status code.
// Returns an error if encoding or writing fails.
//
// This is useful for configuration APIs, DevOps tools, and Kubernetes-style services.
func (c *Context) YAML(code int, obj any) error {
	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(obj)
	if err != nil {
		return fmt.Errorf("YAML encoding failed for type %T: %w", obj, err)
	}

	// Set headers
	c.Response.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")

	// Write status code
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	// Write YAML
	_, writeErr := c.Response.Write(yamlBytes)

	return writeErr
}

// DataFromReader streams data from an io.Reader to the response.
// This is useful for large responses, streaming logs, or real-time data.
//
// Uses io.Copy() for streaming.
// Suitable for responses that should be streamed rather than buffered.
//
// Parameters:
//   - code: HTTP status code
//   - contentLength: Response size in bytes (set to -1 if unknown)
//   - contentType: MIME type (e.g., "application/octet-stream", "text/plain")
//   - reader: Data source to stream from
//   - extraHeaders: Optional additional headers to set
//
// Example:
//
//	// Stream a large file
//	file, _ := os.Open("large-file.bin")
//	defer file.Close()
//	stat, _ := file.Stat()
//	c.DataFromReader(200, stat.Size(), "application/octet-stream", file, nil)
//
//	// Stream with custom headers
//	:= map[string]string{
//	    "Content-Disposition": `attachment; filename="data.bin"`,
//	    "Cache-Control": "no-cache",
//	}
//	c.DataFromReader(200, -1, "application/octet-stream", dataReader, headers)
func (c *Context) DataFromReader(code int, contentLength int64, contentType string, reader io.Reader, extraHeaders map[string]string) error {
	// Set Content-Type
	if contentType != "" {
		c.Response.Header().Set("Content-Type", contentType)
	}

	// Set Content-Length if known
	if contentLength >= 0 {
		c.Response.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	}

	// Set extra headers
	for key, value := range extraHeaders {
		c.Response.Header().Set(key, value)
	}

	// Write status code
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	// Stream data using zero-copy io.Copy
	_, err := io.Copy(c.Response, reader)
	if err != nil {
		return fmt.Errorf("streaming from reader failed: %w", err)
	}

	return nil
}

// Data sends raw bytes with a custom content type.
// Returns an error if writing fails.
//
// This is useful for sending binary data, images, PDFs, or any custom format.
// Direct byte write with no encoding/formatting.
func (c *Context) Data(code int, contentType string, data []byte) error {
	// Set Content-Type
	if contentType != "" {
		c.Response.Header().Set("Content-Type", contentType)
	} else {
		c.Response.Header().Set("Content-Type", "application/octet-stream")
	}

	// Write status code
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	// Write data directly
	_, err := c.Response.Write(data)
	if err != nil {
		return fmt.Errorf("writing data response: %w", err)
	}

	return nil
}

// Error collects an error without immediately writing a response.
// This allows multiple errors to be collected during request processing
// and handled later by middleware or handlers.
//
// Example:
//
//	func handler(c *router.Context) {
//	    if err := validateUser(c); err != nil {
//	        c.Error(err)
//	    }
//	    if err := validateEmail(c); err != nil {
//	        c.Error(err)
//	    }
//
//	    // Process all errors using standard library functions
//	    if c.HasErrors() {
//	        joinedErr := errors.Join(c.Errors()...)
//	        if errors.Is(joinedErr, ErrFileNotFound) {
//	            // Handle specific error
//	        }
//	        if err := c.JSON(400, map[string]any{"errors": c.Errors()}); err != nil {
//	            c.Logger().Error("failed to write error response", "err", err)
//	        }
//	        return
//	    }
//	}
func (c *Context) Error(err error) {
	if err == nil {
		return
	}
	if c.errors == nil {
		c.errors = make([]error, 0, 4)
	}
	c.errors = append(c.errors, err)
}

// Errors returns all errors collected during request processing.
// Returns nil if no errors were collected.
//
// Users can combine errors using errors.Join() or iterate individually.
//
// Example:
//
//	// Combine all errors
//	joinedErr := errors.Join(c.Errors()...)
//
//	// Iterate individually
//	for _, err := range c.Errors() {
//	    // Process each error
//	}
func (c *Context) Errors() []error {
	if c.errors == nil {
		return nil
	}

	return c.errors
}

// HasErrors returns true if any errors were collected during request processing.
func (c *Context) HasErrors() bool {
	return len(c.errors) > 0
}

// WriteErrorResponse writes a simple HTTP error response.
// Error formatting is handled by app.Context.Error() when router.Context is wrapped.
func (c *Context) WriteErrorResponse(status int, message string) {
	if rw, ok := c.Response.(*responseWriter); !ok || !rw.Written() {
		c.Response.WriteHeader(status)
	}
	if message != "" {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(c.Response, message+"\n")
	}
}

// NotFound writes a 404 Not Found response.
func (c *Context) NotFound() {
	c.WriteErrorResponse(http.StatusNotFound, "Not Found")
}

// MethodNotAllowed writes a 405 Method Not Allowed response.
// Sets the required Allow header per RFC 7231.
func (c *Context) MethodNotAllowed(allowed []string) {
	// Sort for deterministic output
	sort.Strings(allowed)
	c.Header("Allow", strings.Join(allowed, ", "))
	c.WriteErrorResponse(http.StatusMethodNotAllowed, "Method Not Allowed")
}

// RecordMetric records a custom histogram metric by delegating to the metrics recorder.
// Thread-safety depends on the underlying metrics recorder implementation.
func (c *Context) RecordMetric(name string, value float64, attributes ...attribute.KeyValue) {
	if c.metricsRecorder != nil {
		c.metricsRecorder.RecordMetric(c.RequestContext(), name, value, attributes...)
	}
}

// IncrementCounter increments a custom counter metric by delegating to the metrics recorder.
// Thread-safety depends on the underlying metrics recorder implementation.
func (c *Context) IncrementCounter(name string, attributes ...attribute.KeyValue) {
	if c.metricsRecorder != nil {
		c.metricsRecorder.IncrementCounter(c.RequestContext(), name, attributes...)
	}
}

// SetGauge sets a custom gauge metric by delegating to the metrics recorder.
// Thread-safety depends on the underlying metrics recorder implementation.
func (c *Context) SetGauge(name string, value float64, attributes ...attribute.KeyValue) {
	if c.metricsRecorder != nil {
		c.metricsRecorder.SetGauge(c.RequestContext(), name, value, attributes...)
	}
}

// TraceID returns the current trace ID from the active span.
// Returns an empty string if tracing is not active.
func (c *Context) TraceID() string {
	if c.tracingRecorder != nil {
		return c.tracingRecorder.TraceID()
	}

	return ""
}

// SpanID returns the current span ID from the active span.
// Returns an empty string if tracing is not active.
func (c *Context) SpanID() string {
	if c.tracingRecorder != nil {
		return c.tracingRecorder.SpanID()
	}

	return ""
}

// SetSpanAttribute adds an attribute to the current span.
// This is a no-op if tracing is not active.
func (c *Context) SetSpanAttribute(key string, value any) {
	if c.tracingRecorder != nil {
		c.tracingRecorder.SetSpanAttribute(key, value)
	}
}

// AddSpanEvent adds an event to the current span with optional attributes.
// This is a no-op if tracing is not active.
func (c *Context) AddSpanEvent(name string, attrs ...attribute.KeyValue) {
	if c.tracingRecorder != nil {
		c.tracingRecorder.AddSpanEvent(name, attrs...)
	}
}

// RequestContext returns the request's context.Context.
// This is a convenience method for passing to functions expecting context.Context.
//
// Use this method when you need the raw request context for:
//   - Database queries: db.Query(c.RequestContext(), ...)
//   - HTTP client calls: httpClient.Do(c.RequestContext(), req)
//   - Any function expecting context.Context
//
// For tracing-aware context propagation, use TraceContext() instead.
//
// Example:
//
//	// Database query
//	users, err := db.QueryUsers(c.RequestContext())
//
//	// HTTP client call
//	resp, err := httpClient.Get(c.RequestContext(), "https://api.example.com")
//
//	// Long-running operation with cancellation
//	select {
//	case <-c.RequestContext().Done():
//		return // Request canceled
//	case result := <-longOperation():
//		if err := c.JSON(200, result); err != nil {
//		    c.Logger().Error("failed to write response", "err", err)
//		}
//	}
func (c *Context) RequestContext() context.Context {
	if c.Request != nil {
		return c.Request.Context()
	}

	return context.Background()
}

// TraceContext returns the OpenTelemetry trace context.
// This can be used for manual span creation or context propagation.
// If tracing is not enabled, it returns the request context for proper cancellation support.
func (c *Context) TraceContext() context.Context {
	if c.tracingRecorder != nil {
		return c.tracingRecorder.TraceContext()
	}
	// Use request context as parent for proper cancellation support
	return c.RequestContext()
}

// Span returns the OpenTelemetry span for this request, if tracing is enabled.
// Returns nil if tracing is not enabled or no span exists.
func (c *Context) Span() trace.Span {
	return c.span
}

// RoutePattern returns the matched route pattern (e.g., "/users/:id").
// Returns empty string if pattern is not available.
func (c *Context) RoutePattern() string {
	return c.routePattern
}

// SetETag sets an ETag header for the response.
// Supports both strong (default) and weak ETags per RFC 7232.
//
// Example:
//

// BindOptions configures strict JSON binding behavior.
type BindOptions struct {
	MaxBytes   int64 // Maximum request body size (0 = no limit)
	DepthLimit int   // Maximum JSON nesting depth (0 = no limit)
}

// RequireContentType checks if the request Content-Type matches one of the allowed types.
// Returns false and sends a 415 Unsupported Media Type problem if no match.
// Supports suffix matching for patterns like "application/*+json".
//
// Example:
//
//	if !c.RequireContentType("application/json") {
//		return // 415 already sent
//	}
func (c *Context) RequireContentType(allowed ...string) bool {
	ct := c.Request.Header.Get("Content-Type")

	// Only require Content-Type for methods that have bodies
	if ct == "" {
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
			return c.unsupportedMediaTypeProblem("", allowed)
		}

		return true // GET/DELETE don't need Content-Type
	}

	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return c.unsupportedMediaTypeProblem(ct, allowed)
	}

	// For JSON, charset must be utf-8 if specified
	if strings.HasSuffix(mediaType, "json") {
		if charset, ok := params["charset"]; ok {
			if !strings.EqualFold(charset, "utf-8") {
				return c.unsupportedMediaTypeProblem(ct, allowed)
			}
		}
	}

	// Check for exact or suffix match
	for _, a := range allowed {
		if a == mediaType {
			return true
		}
		// Handle application/*+json pattern
		if strings.HasSuffix(a, "/*+json") && strings.HasSuffix(mediaType, "+json") {
			return true
		}
	}

	return c.unsupportedMediaTypeProblem(mediaType, allowed)
}

// unsupportedMediaTypeProblem sends a 415 Unsupported Media Type response.
func (c *Context) unsupportedMediaTypeProblem(_ string, _ []string) bool {
	c.WriteErrorResponse(http.StatusUnsupportedMediaType, "Unsupported Media Type")
	return false
}

// RequireContentTypeJSON checks if the request Content-Type is JSON.
// Returns false and sends a 415 problem if not JSON.
//
// Example:
//
//	if !c.RequireContentTypeJSON() {
//		return // 415 already sent
//	}
func (c *Context) RequireContentTypeJSON() bool {
	return c.RequireContentType("application/json", "application/*+json")
}

// writeJSONDecodeProblem converts JSON decode errors to HTTP error responses.
func (c *Context) writeJSONDecodeProblem(err error) error {
	switch {
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		c.WriteErrorResponse(http.StatusBadRequest, "Malformed JSON: Unexpected end of JSON input")
		return err

	case errors.As(err, new(*json.SyntaxError)):
		c.WriteErrorResponse(http.StatusBadRequest, "Malformed JSON: "+err.Error())
		return err

	case errors.As(err, new(*json.UnmarshalTypeError)):
		ute := err.(*json.UnmarshalTypeError)
		// Valid JSON, wrong types -> 422
		c.WriteErrorResponse(http.StatusUnprocessableEntity, fmt.Sprintf("Invalid type for field %q: expected %s", ute.Field, ute.Type))

		return err

	default:
		errStr := err.Error()
		// Unknown field string from DisallowUnknownFields()
		if field, ok := strings.CutPrefix(errStr, "json: unknown field "); ok {
			field = strings.Trim(field, `"`)
			c.WriteErrorResponse(http.StatusBadRequest, fmt.Sprintf("Unknown field %q", field))

			return err
		}

		// Too large body (http.MaxBytesReader returns this error)
		if strings.Contains(errStr, "request body too large") || strings.Contains(errStr, "http: request body too large") {
			c.WriteErrorResponse(http.StatusRequestEntityTooLarge, "Request body exceeds the maximum allowed size")
			return err
		}

		// Fallback
		c.WriteErrorResponse(http.StatusBadRequest, "Malformed JSON: "+err.Error())

		return err
	}
}

// BindStrict binds JSON with strict validation and size limits.
// Returns an error (already written as RFC 9457 problem) if binding fails.
//
// Features:
//   - Rejects unknown fields (catches typos)
//   - Enforces size limits
//   - Distinguishes 400 (malformed) vs 422 (type errors)
//
// Example:
//
//	var req CreateUserRequest
//	if err := c.BindStrict(&req, router.BindOptions{MaxBytes: 1 << 20}); err != nil {
//		return // Error already written
//	}
func (c *Context) BindStrict(dst any, opt BindOptions) error {
	// 1) Content-Type check
	if !c.RequireContentTypeJSON() {
		return ErrContentTypeNotAllowed
	}

	// 2) Size cap
	if opt.MaxBytes > 0 {
		c.Request.Body = http.MaxBytesReader(c.Response, c.Request.Body, opt.MaxBytes)
	}

	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	dec.UseNumber()

	// 3) Decode exactly one JSON value
	if err := dec.Decode(dst); err != nil {
		return c.writeJSONDecodeProblem(err)
	}

	// 4) No trailing data
	if dec.More() {
		c.WriteErrorResponse(http.StatusBadRequest, "Request body must contain a single JSON value")
		return ErrMultipleJSONValues
	}

	return nil
}

// StreamJSONArray streams a JSON array, processing each item individually.
// Useful for large arrays that should be streamed rather than buffered.
//
// This is a generic function (not a method) due to Go's type parameter limitations.
//
// Example:
//
//	err := router.StreamJSONArray(c, func(item User) error {
//		// Process each user
//		return processUser(item)
//	}, 10000) // Max 10k items
func StreamJSONArray[T any](c *Context, each func(T) error, maxItems int) error {
	if !c.RequireContentTypeJSON() {
		return ErrContentTypeNotAllowed
	}

	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	dec.UseNumber()

	// Expect '['
	tok, err := dec.Token()
	if err != nil {
		return c.writeJSONDecodeProblem(err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		c.WriteErrorResponse(http.StatusBadRequest, "Expected a JSON array")
		return ErrExpectedJSONArray
	}

	count := 0
	for dec.More() {
		count++
		if maxItems > 0 && count > maxItems {
			c.WriteErrorResponse(http.StatusBadRequest, fmt.Sprintf("Array exceeds maximum of %d items", maxItems))
			return fmt.Errorf("%w: %d", ErrArrayExceedsMax, maxItems)
		}

		var v T
		if decodeErr := dec.Decode(&v); decodeErr != nil {
			return c.writeJSONDecodeProblem(decodeErr)
		}

		if eachErr := each(v); eachErr != nil {
			return eachErr
		}
	}

	// Read closing ']'
	if _, tokenErr := dec.Token(); tokenErr != nil {
		return c.writeJSONDecodeProblem(tokenErr)
	}

	return nil
}

// StreamNDJSON streams NDJSON (newline-delimited JSON) objects.
// Each line is a separate JSON object, useful for bulk operations.
//
// This is a generic function (not a method) due to Go's type parameter limitations.
//
// Example:
//
//	err := router.StreamNDJSON(c, func(item User) error {
//		return processUser(item)
//	})
func StreamNDJSON[T any](c *Context, each func(T) error) error {
	if !c.RequireContentType("application/x-ndjson") {
		return ErrContentTypeNotAllowed
	}

	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()
	dec.UseNumber()

	for {
		var v T
		if err := dec.Decode(&v); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return c.writeJSONDecodeProblem(err)
		}

		if err := each(v); err != nil {
			return err
		}
	}

	return nil
}

// reset resets the context to its initial state for reuse.
// This method is used for context pooling.
func (c *Context) reset() {
	// Reset core request fields
	c.Request = nil
	c.Response = nil
	c.handlers = nil
	c.index = -1

	// Reset observability and tracing fields
	c.span = nil
	c.metricsRecorder = nil
	c.tracingRecorder = nil
	c.version = ""
	c.routePattern = ""
	c.logger = nil

	// Reset state flags
	c.aborted = false
	c.errors = nil

	// Clear header parsing cache and return arena to pool
	c.cachedAcceptHeader = ""
	c.cachedAcceptSpecs = nil
	if c.cachedArena != nil {
		c.cachedArena.reset()
		arenaPool.Put(c.cachedArena)
		c.cachedArena = nil
	}

	// Clear parameter arrays - only clear used slots
	// Skip if no parameters were used (common case for static routes)
	if c.paramCount > 0 {
		// Clamp to array size to prevent index out of range
		// (paramCount might be invalid if context was corrupted)
		clearCount := min(c.paramCount, 8)

		// Manual loop for small fixed-size string arrays
		// The fixed array size (8 elements) enables predictable behavior.
		//
		// We use clear() for maps (see map clearing below) as map clearing
		// behaves differently.
		for i := range clearCount {
			c.paramKeys[i] = ""
			c.paramValues[i] = ""
		}
		c.paramCount = 0
	}

	// Clear map if it exists (for >8 params) - use clear() builtin (Go 1.21+)
	if c.Params != nil {
		clear(c.Params)
	}
}

// ParamCount returns the number of parameters stored in the context.
// This is used by the pool package to determine which pool to return the context to.
func (c *Context) ParamCount() int32 {
	return c.paramCount
}

// SetParam sets a parameter at the specified index.
// This implements compiler.ContextParamWriter interface to avoid import cycles.
func (c *Context) SetParam(index int, key, value string) {
	if index < 8 {
		c.paramKeys[index] = key
		c.paramValues[index] = value
	} else {
		if c.Params == nil {
			c.Params = make(map[string]string, 2)
		}
		c.Params[key] = value
	}
}

// SetParamMap sets a parameter in the map (for >8 parameters).
// This implements compiler.ContextParamWriter interface to avoid import cycles.
func (c *Context) SetParamMap(key, value string) {
	if c.Params == nil {
		c.Params = make(map[string]string, 2)
	}
	c.Params[key] = value
}

// SetParamCount sets the parameter count.
// This implements compiler.ContextParamWriter interface to avoid import cycles.
func (c *Context) SetParamCount(count int32) {
	c.paramCount = count
}

// initForRequest initializes the context for a new request.
// This is used in the hot path with minimal field assignments.
func (c *Context) initForRequest(req *http.Request, w http.ResponseWriter, handlers []HandlerFunc, router *Router) {
	c.Request = req
	c.Response = w
	c.handlers = handlers
	c.router = router
	c.index = -1
	c.paramCount = 0
	c.version = "" // Reset version for non-versioned routes

	// NOTE: metricsRecorder is now set by app/observability if needed
	// Handler-level custom metrics work through Context.RecordMetric(), IncrementCounter(), SetGauge()
}

// initForRequestWithParams initializes context WITHOUT resetting parameters.
// Used when parameters have already been extracted (e.g., from compiled route matching).
func (c *Context) initForRequestWithParams(req *http.Request, w http.ResponseWriter, handlers []HandlerFunc, router *Router) {
	c.Request = req
	c.Response = w
	c.handlers = handlers
	c.router = router
	c.index = -1
	c.version = "" // Reset version for non-versioned routes
	// Note: paramCount and param arrays NOT reset - already populated by compiled route matching

	// NOTE: metricsRecorder is now set by app/observability if needed
	// Handler-level custom metrics work through Context.RecordMetric(), IncrementCounter(), SetGauge()
}

// unsafeStringToBytes converts a string to a byte slice.
// This is safe for read-only operations (like writing to http.ResponseWriter)
// but MUST NOT be used if the resulting byte slice will be modified.
//
// #nosec G103 - Intentional unsafe usage
func unsafeStringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
