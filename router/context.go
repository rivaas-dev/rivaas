package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Context represents the context of the current HTTP request with optimizations
// for request handling. It provides access to request/response
// objects, URL parameters, and middleware chain execution.
//
// The Context includes several performance features:
//   - Fast parameter storage using arrays for up to 8 parameters
//   - Efficient parameter lookup for common cases
//   - Response methods with minimal allocations
//   - Efficient middleware chain execution
//   - OpenTelemetry tracing support with minimal overhead
//   - Custom metrics recording capabilities
//   - Route versioning support for API versioning
//
// Context objects are pooled and reused to minimize garbage collection pressure.
// Do not retain references to Context objects beyond the request lifetime.
//
// NOTE: Fields are ordered by size (largest to smallest) for optimal memory layout
// and to minimize padding. This reduces struct size and improves cache efficiency.
//
// TIER 3: Cache-line optimization applied
// Hot fields (Request, Response, handlers) are in first 64 bytes for cache locality
type Context struct {
	// CACHE LINE 1: Hottest fields (accessed on every request) - first 64 bytes
	Request  *http.Request       // The HTTP request object (8B)
	Response http.ResponseWriter // The HTTP response writer (8B)
	handlers []HandlerFunc       // Handler chain for this request (24B: ptr+len+cap)
	router   *Router             // Reference to the router for metrics access (8B)

	// Still in cache line 1 (48 bytes used, 16 remaining)
	index      int32 // Current handler index in the chain (4B)
	paramCount int32 // Number of parameters stored in arrays (4B)
	// 8 bytes padding to cache line boundary

	// CACHE LINE 2: Parameter storage (accessed when params present)
	paramKeys   [8]string // Parameter names (up to 8 parameters) (128B)
	paramValues [8]string // Parameter values (up to 8 parameters) (128B)

	// CACHE LINE 3+: Less frequently accessed fields
	Params          map[string]string      // URL parameters (fallback for >8 params)
	span            trace.Span             // Current OpenTelemetry span
	traceCtx        context.Context        // Trace context for propagation
	metricsRecorder ContextMetricsRecorder // Metrics recorder for this context
	tracingRecorder ContextTracingRecorder // Tracing recorder for this context
	version         string                 // Current API version (e.g., "v1", "v2")
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
// Note: In normal operation, contexts are obtained from a pool for better performance.
// Only use this function when you need to create a context outside the normal request flow.
func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{
		Request:  r,
		Response: w,
		index:    -1,
		router:   nil, // Will be set when needed for metrics
		// No allocations in constructor for performance
	}
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
//	            c.JSON(401, map[string]string{"error": "Unauthorized"})
//	            return // Don't call Next() to stop the chain
//	        }
//	        c.Next() // Continue to next handler
//	    }
//	}
func (c *Context) Next() {
	c.index++
	handlersLen := int32(len(c.handlers))

	// Optimized loop: Pre-compute length, check cancellation only if enabled
	if c.router != nil && c.router.checkCancellation {
		// With cancellation checks (default behavior)
		for c.index < handlersLen {
			// Check for context cancellation to avoid processing cancelled requests
			// This is important for long-running handler chains or I/O operations
			if err := c.Request.Context().Err(); err != nil {
				return // Context cancelled or deadline exceeded
			}
			c.handlers[c.index](c)
			c.index++
		}
	} else {
		// Fast path without cancellation checks (~10ns faster per handler)
		for c.index < handlersLen {
			c.handlers[c.index](c)
			c.index++
		}
	}
}

// Param returns the value of the URL parameter by key.
// This method is efficient with minimal allocations for up to 8 parameters.
//
// For routes with parameters like "/users/:id/posts/:post_id", you can extract
// the parameter values using their names:
//
//	userID := c.Param("id")
//	postID := c.Param("post_id")
//
// Returns an empty string if the parameter doesn't exist.
func (c *Context) Param(key string) string {
	// Fast array lookup first (zero allocations for ≤8 params)
	for i := int32(0); i < c.paramCount; i++ {
		if c.paramKeys[i] == key {
			return c.paramValues[i]
		}
	}
	// Fallback to map for >8 parameters (rare case)
	return c.Params[key]
}

// JSON sends a JSON response with the specified status code.
// The object will be marshaled to JSON and written to the response.
// Returns an error if JSON encoding fails.
//
// This method encodes to a buffer first to catch errors before writing headers,
// ensuring responses are never left in an inconsistent state. This adds a small
// overhead but provides better error handling and reliability.
//
// Example:
//
//	if err := c.JSON(http.StatusOK, map[string]string{"message": "Hello World"}); err != nil {
//		// Handle encoding error (headers not yet written)
//		c.JSON(http.StatusInternalServerError, map[string]string{"error": "encoding failed"})
//		return
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

// MustJSON sends a JSON response with automatic error handling.
// If JSON encoding fails, it automatically sends a 500 error response with details.
// This is the recommended method for handlers that don't need custom error handling.
//
// BREAKING CHANGE: This method now sends detailed error information in development.
// For production, ensure you have appropriate error handling middleware that sanitizes errors.
//
// Example:
//
//	c.MustJSON(http.StatusOK, map[string]string{"message": "Hello World"})
//	c.MustJSON(http.StatusCreated, user)
func (c *Context) MustJSON(code int, obj any) {
	if err := c.JSON(code, obj); err != nil {
		// Send error response - headers haven't been written yet due to buffering
		c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
		c.Response.WriteHeader(http.StatusInternalServerError)

		// Send structured error response
		errorResponse := fmt.Sprintf(`{"error":"JSON encoding failed","type":"%T","details":"%s"}`,
			obj, err.Error())
		c.Response.Write([]byte(errorResponse))
	}
}

// String sends a plain text response with optional formatting.
// This method minimizes allocations for common patterns.
// Returns an error if writing to the response fails.
//
// For simple strings without formatting, zero allocations are achieved:
//
//	c.String(200, "Hello World")              // Zero allocations
//	c.String(200, "User: %s", username)       // Efficient for single %s
//	c.String(200, "Complex: %d %s", id, name) // Falls back to fmt.Fprintf
//
// The method automatically optimizes single %s patterns when exactly one value is provided.
func (c *Context) String(code int, format string, values ...any) error {
	if c.Response.Header().Get("Content-Type") == "" {
		c.Response.Header().Set("Content-Type", "text/plain")
	}

	// Check if headers have already been written to avoid "superfluous response.WriteHeader call"
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	// Zero allocations for plain strings
	if len(values) == 0 {
		_, err := c.Response.Write([]byte(format))
		if err != nil {
			return fmt.Errorf("writing string response: %w", err)
		}
		return nil
	}

	// Optimize single %s pattern with single string value (common case)
	// Only applies when: 1 value, value is string, exactly 1 %s in format
	if len(values) == 1 {
		if v, ok := values[0].(string); ok && strings.Count(format, "%s") == 1 {
			idx := strings.Index(format, "%s")
			if idx != -1 {
				// Use string concatenation to avoid []byte allocations
				result := format[:idx] + v + format[idx+2:]
				_, err := c.Response.Write([]byte(result))
				if err != nil {
					return fmt.Errorf("writing formatted string response: %w", err)
				}
				return nil
			}
		}
	}

	// Fallback for complex formatting (multiple values, non-string types, etc.)
	// Direct fmt.Fprintf to response - eliminates 2 allocations
	_, err := fmt.Fprintf(c.Response, format, values...)
	if err != nil {
		return fmt.Errorf("writing formatted string response: %w", err)
	}
	return nil
}

// HTML sends an HTML response with the specified status code.
// Returns an error if writing to the response fails.
//
// Example:
//
//	c.HTML(200, "<h1>Welcome</h1>")
//	c.HTML(404, "<h1>Page Not Found</h1>")
func (c *Context) HTML(code int, html string) error {
	c.Response.Header().Set("Content-Type", "text/html")
	c.Response.WriteHeader(code)
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
		// Log security event if logger is configured
		if c.router != nil && c.router.logger != nil {
			c.router.logger.Warn("header injection attempt blocked and sanitized",
				"key", key,
				"original_value", value,
				"path", c.Request.URL.Path,
				"client_ip", c.ClientIP(),
				"user_agent", c.Request.UserAgent(),
			)
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

// File serves a file from the filesystem to the client.
// This handles proper content types efficiently.
//
// Example:
//
//	c.File("./uploads/document.pdf")
//	c.File("/var/www/static/image.jpg")
func (c *Context) File(filepath string) {
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
	return c.version
}

// IsVersion returns true if the current request is for the specified version.
// This is an efficient helper for version checking.
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

// RecordMetric records a custom histogram metric with the given name and value.
// This method is thread-safe and uses atomic operations for optimal performance.
func (c *Context) RecordMetric(name string, value float64, attributes ...attribute.KeyValue) {
	if c.metricsRecorder != nil {
		c.metricsRecorder.RecordMetric(c.Request.Context(), name, value, attributes...)
	}
}

// IncrementCounter increments a custom counter metric with the given name.
// This method is thread-safe and uses atomic operations for optimal performance.
func (c *Context) IncrementCounter(name string, attributes ...attribute.KeyValue) {
	if c.metricsRecorder != nil {
		c.metricsRecorder.IncrementCounter(c.Request.Context(), name, attributes...)
	}
}

// SetGauge sets a custom gauge metric with the given name and value.
// This method is thread-safe and uses atomic operations for optimal performance.
func (c *Context) SetGauge(name string, value float64, attributes ...attribute.KeyValue) {
	if c.metricsRecorder != nil {
		c.metricsRecorder.SetGauge(c.Request.Context(), name, value, attributes...)
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
func (c *Context) SetSpanAttribute(key string, value interface{}) {
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

// TraceContext returns the OpenTelemetry trace context.
// This can be used for manual span creation or context propagation.
// If tracing is not enabled, it returns the request context for proper cancellation support.
func (c *Context) TraceContext() context.Context {
	if c.tracingRecorder != nil {
		return c.tracingRecorder.TraceContext()
	}
	// Use request context as parent for proper cancellation support
	if c.Request != nil {
		return c.Request.Context()
	}
	return context.Background()
}

// reset resets the context to its initial state for reuse.
// This method is efficient for context pooling with minimal allocations.
func (c *Context) reset() {
	// Fast reset without allocations
	c.Request = nil
	c.Response = nil
	c.handlers = nil
	c.index = -1
	c.version = ""
	c.span = nil
	c.traceCtx = nil
	c.metricsRecorder = nil
	c.tracingRecorder = nil

	// Clear parameter arrays efficiently - only clear used slots
	// Optimization: Skip if no parameters were used (common case for static routes)
	if c.paramCount > 0 {
		// Use range for cleaner code (Go 1.22+)
		for i := range c.paramCount {
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

// initForRequest initializes the context for a new request.
// This is optimized for the hot path with minimal field assignments.
// Inlining this function reduces call overhead in ServeHTTP.
func (c *Context) initForRequest(req *http.Request, w http.ResponseWriter, handlers []HandlerFunc, router *Router) {
	c.Request = req
	c.Response = w
	c.handlers = handlers
	c.router = router
	c.index = -1
	c.paramCount = 0
}

// initForRequestWithParams initializes context WITHOUT resetting parameters.
// Used when parameters have already been extracted (e.g., from template matching).
func (c *Context) initForRequestWithParams(req *http.Request, w http.ResponseWriter, handlers []HandlerFunc, router *Router) {
	c.Request = req
	c.Response = w
	c.handlers = handlers
	c.router = router
	c.index = -1
	// Note: paramCount and param arrays NOT reset - already populated by template
}
