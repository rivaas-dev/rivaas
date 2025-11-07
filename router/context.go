package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync/atomic"
	"unsafe"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
	"rivaas.dev/logging"
	"rivaas.dev/router/middleware"
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
// ⚠️ MEMORY SAFETY: Context objects are pooled and reused to minimize garbage collection pressure.
//
// CRITICAL RULES:
//  1. DO NOT retain references to Context objects beyond the request handler lifetime.
//  2. If you MUST retain a reference (e.g., async operations), call Release() when done.
//  3. DO NOT use a Context after calling Release() - it will be reused for other requests.
//  4. The router automatically returns contexts to the pool after request completion.
//
// Why this matters:
//   - Contexts are reused across requests to reduce allocations
//   - Retaining references causes memory leaks and data corruption
//   - Use-after-release causes undefined behavior and security issues
//
// Example (CORRECT - no manual release needed):
//
//	func handler(c *router.Context) {
//	    userID := c.Param("id")
//	    c.JSON(200, map[string]string{"id": userID})
//	    // Context automatically returned to pool by router
//	}
//
// Example (CORRECT - async operation with release):
//
//	func handler(c *router.Context) {
//	    go func(ctx *router.Context) {
//	        defer ctx.Release() // CRITICAL: Release when done
//	        // Process async work...
//	    }(c)
//	}
//
// Example (WRONG - retaining reference without release):
//
//	var globalContext *router.Context // BAD!
//
//	func handler(c *router.Context) {
//	    globalContext = c // BAD! Memory leak and data corruption
//	}
//
// See Release() method for more details on manual context management.
//
// NOTE: Fields are ordered by size (largest to smallest) for optimal memory layout
// and to minimize padding. This reduces struct size and improves cache efficiency.
//
// Hot fields (Request, Response, handlers) are placed in the first 64 bytes
// for better cache locality during request processing.
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
	metricsRecorder ContextMetricsRecorder // Metrics recorder for this context
	tracingRecorder ContextTracingRecorder // Tracing recorder for this context
	version         string                 // Current API version (e.g., "v1", "v2")
	routeTemplate   string                 // Matched route template (e.g., "/users/:id" or "_not_found")

	// Header parsing cache (per-request)
	cachedAcceptHeader string       // Cached Accept header value
	cachedAcceptSpecs  []acceptSpec // Parsed Accept header specs
	cachedArena        *headerArena // Arena allocator for spec buffers (pooled)

	// Binding metadata (per-request)
	bindingMeta *bindingMetadata // Cached body, presence tracking, etc.

	// Abort flag to stop handler chain execution
	aborted bool // Set to true when Abort() is called

	// Memory safety: Track if context has been released to prevent use-after-release
	// Uses atomic.Bool for thread-safe access when context is used from multiple goroutines
	released atomic.Bool // Set to true when Release() is called - DO NOT use after this
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
			// Check for context cancellation to avoid processing cancelled requests
			// This is important for long-running handler chains or I/O operations
			if err := c.Request.Context().Err(); err != nil {
				return // Context cancelled or deadline exceeded
			}
			c.handlers[c.index](c)
			c.index++
		}
	} else {
		// Fast path without cancellation checks (slightly faster per handler)
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
//	            c.JSON(401, map[string]string{"error": "Unauthorized"})
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
// This method is efficient with minimal allocations for up to 8 parameters.
//
// For routes with parameters like "/users/:id/posts/:post_id", you can extract
// the parameter values using their names:
//
//	userID := c.Param("id")
//	postID := c.Param("post_id")
//
// Returns an empty string if the parameter doesn't exist.
//
// Compiler hint: This is a hot path function that should be inlined for performance.
// The small size and frequent usage make it an ideal candidate for inlining.
//
//go:inline
func (c *Context) Param(key string) string {
	// Safety check: prevent use-after-release (thread-safe atomic read)
	if c.released.Load() {
		return ""
	}
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
	// Safety check: prevent use-after-release (thread-safe atomic read)
	if c.released.Load() {
		return ErrContextReleased
	}
	// Encode to buffer first to catch errors before writing headers
	// This prevents inconsistent response state if encoding fails
	var buf strings.Builder
	buf.Grow(256) // Pre-allocate reasonable size for most JSON responses

	if err := json.NewEncoder(&buf).Encode(obj); err != nil {
		// Return error without writing anything - caller can handle it
		return fmt.Errorf("JSON encoding failed for type %T: %w", obj, err)
	}

	// Only write headers after successful encoding
	// Safety check: Response may be nil if context was released
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
// This is useful for debugging, development, and human-readable API responses.
//
// Performance: ~30-50% slower than JSON() due to formatting overhead.
// Do NOT use in high-frequency endpoints (>1K req/s). Use JSON() instead.
//
// Example:
//
//	// Development/debugging endpoint
//	c.IndentedJSON(http.StatusOK, user)
//	// Output:
//	// {
//	//   "id": 123,
//	//   "name": "John"
//	// }
//
//	// Production: Use JSON() instead for better performance
//	c.JSON(http.StatusOK, user)
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
// Unlike JSON(), this does not escape <, >, &, and other HTML characters.
//
// Performance: Identical to JSON() - only changes encoder flag.
// Safe for production use when HTML escaping breaks functionality.
//
// Use cases:
//   - Responses containing HTML/markdown content
//   - URLs with query parameters
//   - Code snippets in JSON responses
//
// Example:
//
//	data := map[string]string{
//	    "html": "<h1>Title</h1>",
//	    "url":  "https://example.com?foo=bar&baz=qux",
//	}
//	c.PureJSON(200, data)
//	// Output: {"html":"<h1>Title</h1>","url":"https://example.com?foo=bar&baz=qux"}
//
//	// Compare with JSON() which would escape:
//	// {"html":"\u003ch1\u003eTitle\u003c/h1\u003e","url":"https://example.com?foo=bar\u0026baz=qux"}
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
// The prefix prevents the response from being executed as JavaScript in old browsers.
//
// Performance: <1% overhead - just prepends prefix string.
// Safe for production use with minimal performance impact.
//
// Default prefix: "while(1);" (matches Gin's default)
// The client must strip this prefix before parsing JSON.
//
// Background: Prevents ancient JSON hijacking attack where malicious sites
// could override Array constructor and steal JSON array responses via <script> tags.
// Modern browsers are not vulnerable, but some compliance requirements still mandate this.
//
// Example:
//
//	c.SecureJSON(200, []string{"secret1", "secret2"})
//	// Output: while(1);["secret1","secret2"]
//
//	c.SecureJSON(200, data, "for(;;);")
//	// Output: for(;;);{"key":"value"}
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
// This ensures the response is pure ASCII, useful for legacy systems or strict compatibility.
//
// Performance: +10-15% overhead vs JSON() due to Unicode escaping.
// Only use when legacy client compatibility requires pure ASCII.
//
// All non-ASCII characters (including emoji, Chinese, Japanese, etc.) are escaped
// to their Unicode code point representation (\uXXXX).
//
// Example:
//
//	data := map[string]string{
//	    "message": "Hello 世界 🌍",
//	    "name":    "José",
//	}
//	c.ASCIIJSON(200, data)
//	// Output: {"message":"Hello \u4e16\u754c \ud83c\udf0d","name":"Jos\u00e9"}
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
				// Write directly to response in chunks to avoid intermediate allocations
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
	// Safety check: prevent use-after-release (thread-safe atomic read)
	if c.released.Load() || c.Request == nil {
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
	// Lazy version detection
	// Only detect version when first accessed, not on every request
	if c.version == "" && c.router.versioning != nil {
		c.version = c.router.detectVersion(c.Request)
	}
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

// YAML sends a YAML response with the specified status code.
// This is useful for configuration APIs, DevOps tools, and Kubernetes-style services.
//
// Performance: +150-300% overhead vs JSON() due to YAML marshaling complexity.
// Do NOT use in high-frequency endpoints. Reserve for config/admin APIs only.
//
// Requires: gopkg.in/yaml.v3 dependency
//
// Example:
//
//	config := map[string]interface{}{
//	    "database": map[string]string{
//	        "host": "localhost",
//	        "port": "5432",
//	    },
//	    "debug": true,
//	}
//	c.YAML(200, config)
//	// Output:
//	// database:
//	//   host: localhost
//	//   port: "5432"
//	// debug: true
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
// This is efficient for large responses, streaming logs, or real-time data.
//
// Performance: Zero-copy streaming with io.Copy().
// Excellent for large responses (>100KB) - avoids buffering entire response in memory.
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
//	headers := map[string]string{
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
		c.Response.Header().Set("Content-Length", fmt.Sprintf("%d", contentLength))
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
// This is useful for sending binary data, images, PDFs, or any custom format.
//
// Performance: ~0% overhead - direct byte write with no encoding.
// Safe for all use cases including high-frequency endpoints.
//
// Example:
//
//	// Send PNG image
//	imageData := loadImage()
//	c.Data(200, "image/png", imageData)
//
//	// Send PDF
//	pdfData := generatePDF()
//	c.Data(200, "application/pdf", pdfData)
//
//	// Send custom binary format
//	c.Data(200, "application/octet-stream", binaryData)
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

// ProblemDetail writes an RFC 9457 Problem Details response.
// Automatically negotiates content type, injects observability metadata,
// and sets appropriate headers.
//
// RFC 9457 alignment:
//   - Prefers application/problem+json content type
//   - Adds Link header for non-blank types
//   - Enforces 4xx/5xx status codes
//   - Auto-injects trace_id and request_id for correlation
//
// Example:
//
//	problem := router.NewProblemDetail(404, "User Not Found").
//		WithType(router.ProblemTypeNotFound).
//		WithDetail("User with ID 12345 does not exist").
//		WithExtension("user_id", "12345")
//	return c.ProblemDetail(problem)
func (c *Context) ProblemDetail(p *ProblemDetail) error {
	if p == nil {
		return ErrProblemDetailNil
	}

	// RFC 9457: Set defaults
	if p.Type == "" {
		p.Type = PTBlank
	}
	if p.Title == "" {
		if txt := http.StatusText(p.Status); txt != "" {
			p.Title = txt
		} else {
			p.Title = "Error"
		}
	}

	// RFC 9457: Problems are for errors (4xx/5xx), not success
	if p.Status < 400 || p.Status >= 600 {
		c.LogWarn("invalid problem status",
			"status", p.Status,
			"path", c.Request.URL.Path,
		)
		p.Status = http.StatusInternalServerError
	}

	// Auto-inject observability metadata (safe - checks for duplicates)
	if p.Extensions == nil {
		p.Extensions = make(map[string]any, 2)
	}

	// OpenTelemetry trace ID
	if tid := c.TraceID(); tid != "" {
		if _, exists := p.Extensions["trace_id"]; !exists {
			p.Extensions["trace_id"] = tid
		}
	}

	// Request ID from middleware (type-safe)
	if v := c.Request.Context().Value(middleware.RequestIDKey); v != nil {
		if rid, ok := v.(string); ok && rid != "" {
			if _, exists := p.Extensions["request_id"]; !exists {
				p.Extensions["request_id"] = rid
			}
		}
	}

	// RFC 9457: Link header for problem type documentation
	if p.Type != "" && p.Type != PTBlank {
		c.AppendHeader("Link", fmt.Sprintf("<%s>; rel=\"describedby\"", p.Type))
	}

	// Mark span on 5xx errors (server errors indicate problems worth investigating)
	// Note: OTel doesn't expose span.Status() for reading, so we can't check
	// if error was already set. Setting codes.Error multiple times is harmless.
	if p.Status >= 500 && c.span != nil && c.span.SpanContext().IsValid() {
		// Set error status (safe to call multiple times)
		c.span.SetStatus(codes.Error, http.StatusText(p.Status))

		// Add problem detail attributes
		c.span.SetAttributes(
			attribute.String("http.problem.type", p.Type),
			attribute.Int("http.problem.status", p.Status),
		)

		// Add error_id if present
		if errorID, ok := p.Extensions["error_id"].(string); ok && errorID != "" {
			c.span.SetAttributes(attribute.String("error_id", errorID))
		}

		// Record error if we have an underlying cause (via WithCause)
		if p.cause != nil {
			c.span.RecordError(p.cause)
		}

		// NOTE: exception.escaped is NOT set here - that's only for panics
	}

	// Content negotiation (RFC 9457 prefers application/problem+json)
	offer := c.Accepts("application/problem+json", "application/json", "text/plain")

	switch offer {
	case "text/plain":
		// Minimal text representation for curl/simple clients
		c.Header("Content-Type", "text/plain; charset=utf-8")
		if rw, ok := c.Response.(*responseWriter); !ok || !rw.Written() {
			c.Response.WriteHeader(p.Status)
		}

		// Simple format: Title + Detail (no sensitive internals)
		text := p.Title
		if p.Detail != "" {
			text += ": " + p.Detail
		}
		text += "\n"

		_, err := io.WriteString(c.Response, text)
		return err

	default:
		// JSON responses (application/problem+json or application/json)
		ct := MediaTypeProblemJSON
		if offer == "application/json" {
			ct = "application/json; charset=utf-8"
		}

		// Use pooled buffer to reduce allocations
		buf := bufferPool.Get().(*bytes.Buffer)
		buf.Reset()
		defer bufferPool.Put(buf)

		enc := json.NewEncoder(buf)
		if err := enc.Encode(p); err != nil {
			return fmt.Errorf("ProblemDetail encoding failed: %w", err)
		}

		c.Header("Content-Type", ct)
		if rw, ok := c.Response.(*responseWriter); !ok || !rw.Written() {
			c.Response.WriteHeader(p.Status)
		}

		_, err := c.Response.Write(buf.Bytes())
		return err
	}
}

// Problem is a convenience to build and write a problem in one call.
// Useful for simple error cases without needing the builder pattern.
//
// Example:
//
//	return c.Problem(404, router.ProblemTypeNotFound, "User Not Found",
//		"User with the given ID does not exist",
//		map[string]any{"user_id": userID})
func (c *Context) Problem(status int, typeURI, title, detail string, ext map[string]any) error {
	p := NewProblemDetail(status, title).
		WithType(typeURI).
		WithDetail(detail).
		WithInstance(c.Request.URL.Path)

	if len(ext) > 0 {
		p.WithExtensions(ext)
	}

	return c.ProblemDetail(p)
}

// NotFoundProblem sends a 404 Not Found problem response.
// This is the standard RFC 9457 response for unmatched routes.
//
// Example:
//
//	return c.NotFoundProblem()
func (c *Context) NotFoundProblem() error {
	return c.ProblemDetail(
		NewProblemDetail(http.StatusNotFound, "Not Found").
			WithType(c.ProblemType(PTNotFound)).
			WithInstance(c.Request.URL.Path),
	)
}

// MethodNotAllowedProblem sends a 405 Method Not Allowed problem response.
// This includes the required Allow header and lists allowed methods in the response.
//
// RFC 7231 requires the Allow header to be set for 405 responses.
//
// Example:
//
//	allowed := []string{"GET", "POST"}
//	return c.MethodNotAllowedProblem(allowed)
func (c *Context) MethodNotAllowedProblem(allowed []string) error {
	// Sort for deterministic output
	sort.Strings(allowed)

	// RFC 7231: Must set Allow header
	c.Header("Allow", strings.Join(allowed, ", "))

	return c.ProblemDetail(
		NewProblemDetail(http.StatusMethodNotAllowed, "Method Not Allowed").
			WithType(c.ProblemType(PTMethodNotAllowed)).
			WithDetail(fmt.Sprintf("The %s method is not allowed for this resource.", c.Request.Method)).
			WithInstance(c.Request.URL.Path).
			WithExtension("allowed_methods", allowed),
	)
}

// UnauthorizedProblem sends a 401 Unauthorized problem response.
func (c *Context) UnauthorizedProblem(detail string) error {
	return c.Problem(http.StatusUnauthorized, c.ProblemType(PTUnauthorized), "Unauthorized", detail, nil)
}

// ForbiddenProblem sends a 403 Forbidden problem response.
func (c *Context) ForbiddenProblem(detail string) error {
	return c.Problem(http.StatusForbidden, c.ProblemType(PTForbidden), "Forbidden", detail, nil)
}

// ConflictProblem sends a 409 Conflict problem response.
func (c *Context) ConflictProblem(detail string) error {
	return c.Problem(http.StatusConflict, c.ProblemType(PTConflict), "Conflict", detail, nil)
}

// InternalProblem sends a 500 Internal Server Error problem response.
func (c *Context) InternalProblem(detail string) error {
	return c.Problem(http.StatusInternalServerError, c.ProblemType(PTInternal), "Internal Server Error", detail, nil)
}

// ProblemType resolves a problem type slug to a full URI.
// If the slug is already an absolute URI (starts with "http"), it is returned as-is.
// If a base URL is configured, the slug is appended to it.
// Otherwise, returns "about:blank".
//
// Example:
//
//	// With base URL "https://docs.rivaas.dev/problems":
//	c.ProblemType("validation-error") // Returns "https://docs.rivaas.dev/problems/validation-error"
//	c.ProblemType("https://api.example.com/problems/custom") // Returns as-is
//	c.ProblemType("") // Returns "about:blank"
func (c *Context) ProblemType(slug string) string {
	// Already absolute
	if strings.HasPrefix(slug, "http://") || strings.HasPrefix(slug, "https://") {
		return slug
	}

	// No base configured or empty slug
	if c.router.problemBase == "" || slug == "" {
		return "about:blank"
	}

	// Resolve relative slug
	return c.router.problemBase + "/" + slug
}

// RecordMetric records a custom histogram metric by delegating to the metrics recorder.
// Thread-safety depends on the underlying metrics recorder implementation.
func (c *Context) RecordMetric(name string, value float64, attributes ...attribute.KeyValue) {
	if c.metricsRecorder != nil {
		c.metricsRecorder.RecordMetric(c.Request.Context(), name, value, attributes...)
	}
}

// IncrementCounter increments a custom counter metric by delegating to the metrics recorder.
// Thread-safety depends on the underlying metrics recorder implementation.
func (c *Context) IncrementCounter(name string, attributes ...attribute.KeyValue) {
	if c.metricsRecorder != nil {
		c.metricsRecorder.IncrementCounter(c.Request.Context(), name, attributes...)
	}
}

// SetGauge sets a custom gauge metric by delegating to the metrics recorder.
// Thread-safety depends on the underlying metrics recorder implementation.
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

// Span returns the OpenTelemetry span for this request, if tracing is enabled.
// Returns nil if tracing is not enabled or no span exists.
func (c *Context) Span() trace.Span {
	return c.span
}

// RouteTemplate returns the matched route template (e.g., "/users/:id").
// Returns empty string if template is not available.
func (c *Context) RouteTemplate() string {
	return c.routeTemplate
}

// Logger returns the router's logger if available.
// Returns nil if no logger is configured.
//
// Example:
//
//	if logger := c.Logger(); logger != nil {
//	    logger.Info("processing request", "user_id", userID)
//	}
func (c *Context) Logger() logging.Logger {
	if c.router != nil {
		return c.router.logger
	}
	return nil
}

// LogDebug logs a debug message using the router's logger.
// This is a no-op if no logger is configured.
func (c *Context) LogDebug(msg string, args ...any) {
	if logger := c.Logger(); logger != nil {
		logger.Debug(msg, args...)
	}
}

// LogInfo logs an info message using the router's logger.
// This is a no-op if no logger is configured.
func (c *Context) LogInfo(msg string, args ...any) {
	if logger := c.Logger(); logger != nil {
		logger.Info(msg, args...)
	}
}

// LogWarn logs a warning message using the router's logger.
// This is a no-op if no logger is configured.
func (c *Context) LogWarn(msg string, args ...any) {
	if logger := c.Logger(); logger != nil {
		logger.Warn(msg, args...)
	}
}

// LogError logs an error message using the router's logger.
// This is a no-op if no logger is configured.
func (c *Context) LogError(msg string, args ...any) {
	if logger := c.Logger(); logger != nil {
		logger.Error(msg, args...)
	}
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
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
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

// unsupportedMediaTypeProblem sends a 415 Unsupported Media Type problem.
func (c *Context) unsupportedMediaTypeProblem(received string, allowed []string) bool {
	_ = c.ProblemDetail(
		NewProblemDetail(http.StatusUnsupportedMediaType, "Unsupported Media Type").
			WithType(c.ProblemType(PTUnsupportedMediaType)).
			WithDetail("This endpoint only accepts specific media types.").
			WithExtension("received", received).
			WithExtension("supported_types", allowed),
	)
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

// writeJSONDecodeProblem converts JSON decode errors to RFC 9457 problems.
func (c *Context) writeJSONDecodeProblem(err error) error {
	switch {
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return c.ProblemDetail(
			NewProblemDetail(http.StatusBadRequest, "Malformed JSON").
				WithType(c.ProblemType(PTMalformedJSON)).
				WithDetail("Unexpected end of JSON input."),
		)

	case errors.As(err, new(*json.SyntaxError)):
		return c.ProblemDetail(
			NewProblemDetail(http.StatusBadRequest, "Malformed JSON").
				WithType(c.ProblemType(PTMalformedJSON)).
				WithDetail(err.Error()),
		)

	case errors.As(err, new(*json.UnmarshalTypeError)):
		ute := err.(*json.UnmarshalTypeError)
		// Valid JSON, wrong types -> 422
		return c.ProblemDetail(
			NewProblemDetail(http.StatusUnprocessableEntity, "Unprocessable Entity").
				WithType(c.ProblemType(PTValidation)).
				WithDetail(fmt.Sprintf("Invalid type for field %q: expected %s.", ute.Field, ute.Type)).
				WithExtension("field", ute.Field).
				WithExtension("expected_type", ute.Type.String()),
		)

	default:
		errStr := err.Error()
		// Unknown field string from DisallowUnknownFields()
		if field, ok := strings.CutPrefix(errStr, "json: unknown field "); ok {
			field = strings.Trim(field, `"`)
			return c.ProblemDetail(
				NewProblemDetail(http.StatusBadRequest, "Malformed JSON").
					WithType(c.ProblemType(PTMalformedJSON)).
					WithDetail(fmt.Sprintf("Unknown field %q.", field)).
					WithExtension("unknown_field", field),
			)
		}

		// Too large body (http.MaxBytesReader returns this error)
		if strings.Contains(errStr, "request body too large") || strings.Contains(errStr, "http: request body too large") {
			return c.ProblemDetail(
				NewProblemDetail(http.StatusRequestEntityTooLarge, "Payload Too Large").
					WithType(c.ProblemType(PTTooLarge)).
					WithDetail("Request body exceeds the maximum allowed size."),
			)
		}

		// Fallback
		return c.ProblemDetail(
			NewProblemDetail(http.StatusBadRequest, "Malformed JSON").
				WithType(c.ProblemType(PTMalformedJSON)).
				WithDetail(err.Error()),
		)
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
		return c.ProblemDetail(
			NewProblemDetail(http.StatusBadRequest, "Malformed JSON").
				WithType(c.ProblemType(PTMalformedJSON)).
				WithDetail("Request body must contain a single JSON value."),
		)
	}

	return nil
}

// StreamJSONArray streams a JSON array, processing each item individually.
// Useful for large arrays that shouldn't be loaded entirely into memory.
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
		return c.ProblemDetail(
			NewProblemDetail(http.StatusBadRequest, "Malformed JSON").
				WithType(c.ProblemType(PTMalformedJSON)).
				WithDetail("Expected a JSON array."),
		)
	}

	count := 0
	for dec.More() {
		count++
		if maxItems > 0 && count > maxItems {
			return c.ProblemDetail(
				NewProblemDetail(http.StatusBadRequest, "Too Many Items").
					WithType(c.ProblemType(PTTooLarge)).
					WithDetail(fmt.Sprintf("Array exceeds maximum of %d items", maxItems)),
			)
		}

		var v T
		if err := dec.Decode(&v); err != nil {
			return c.writeJSONDecodeProblem(err)
		}

		if err := each(v); err != nil {
			return err
		}
	}

	// Read closing ']'
	if _, err := dec.Token(); err != nil {
		return c.writeJSONDecodeProblem(err)
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
// This method is efficient for context pooling with minimal allocations.
func (c *Context) reset() {
	// Fast reset without allocations
	c.Request = nil
	c.Response = nil
	c.handlers = nil
	c.index = -1
	c.version = ""
	c.span = nil
	c.metricsRecorder = nil
	c.tracingRecorder = nil
	c.aborted = false

	// Clear header parsing cache and return arena to pool
	c.cachedAcceptHeader = ""
	c.cachedAcceptSpecs = nil
	if c.cachedArena != nil {
		c.cachedArena.reset()
		arenaPool.Put(c.cachedArena)
		c.cachedArena = nil
	}

	// Clear binding metadata cache
	c.bindingMeta = nil

	// Clear parameter arrays efficiently - only clear used slots
	// Optimization: Skip if no parameters were used (common case for static routes)
	if c.paramCount > 0 {
		// Clamp to array size to prevent index out of range
		// (paramCount might be invalid if context was corrupted)
		clearCount := min(c.paramCount, 8)

		// Performance note: Manual loop vs clear() builtin (Go 1.21+)
		// Benchmarks show manual loop is 4-10x faster for small string arrays:
		//   - 8 elements: manual=1.1ns, clear()=5.3ns
		//   - 2 elements: manual=0.46ns, clear()=4.7ns
		//
		// While clear() is more idiomatic, the manual loop provides measurably
		// better performance for this hot path. The compiler can optimize the
		// tight loop very effectively, and the fixed array size enables further
		// optimizations. For reference, this optimization saves ~4ns per reset,
		// which is meaningful when processing millions of requests per second.
		//
		// We DO use clear() for maps (line 1117) as map clearing has different
		// performance characteristics where clear() is optimized.
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

	// Reset observability fields
	c.version = ""
	c.routeTemplate = ""
	c.aborted = false
	c.released.Store(false) // Reset released flag for reuse (thread-safe atomic write)
}

// Release marks the context as invalid and returns it to the pool.
//
// ⚠️ CRITICAL: DO NOT use the context after calling Release().
// The context will be reused for other requests, and any retained references
// will point to invalid data, causing memory safety issues and data corruption.
//
// This method:
//   - Clears all sensitive data (Request, Response, binding metadata, etc.)
//   - Marks the context as released to prevent accidental reuse
//   - Returns the context to the appropriate pool for reuse
//
// When to use Release():
//   - You've stored a reference to Context beyond the request handler
//   - You're using Context in async operations (goroutines, channels)
//   - You need to explicitly return a context to the pool before handler completion
//
// ⚠️ WARNING: In normal request handling, you should NOT call Release() manually.
// The router automatically returns contexts to the pool after request completion.
// Only call Release() if you've retained a Context reference beyond the handler.
//
// Example (async operation):
//
//	func handler(c *router.Context) {
//	    // Start async operation
//	    go func(ctx *router.Context) {
//	        defer ctx.Release() // CRITICAL: Release when done
//	        // Process async work...
//	    }(c)
//	}
//
// Example (stored reference):
//
//	func handler(c *router.Context) {
//	    // Store reference for later use
//	    storedContext := c
//	    // ... later, when done ...
//	    storedContext.Release() // CRITICAL: Release when done
//	}
//
// Memory Safety:
//   - Contexts are pooled and reused to minimize GC pressure
//   - Retaining references beyond request lifetime causes memory leaks
//   - Use-after-release causes undefined behavior and data corruption
//   - Always call Release() if you retain a Context reference
func (c *Context) Release() {
	// Thread-safe check: prevent double release using atomic compare-and-swap
	// This ensures only one goroutine can successfully mark the context as released
	if c.released.Swap(true) {
		// Already released - prevent double release
		return
	}

	// Clear sensitive data first
	c.Request = nil
	c.Response = nil
	c.handlers = nil
	c.bindingMeta = nil
	c.span = nil
	c.metricsRecorder = nil
	c.tracingRecorder = nil

	// Clear header parsing cache
	c.cachedAcceptHeader = ""
	c.cachedAcceptSpecs = nil
	if c.cachedArena != nil {
		c.cachedArena.reset()
		arenaPool.Put(c.cachedArena)
		c.cachedArena = nil
	}

	// Clear parameters
	if c.paramCount > 0 {
		clearCount := min(c.paramCount, 8)
		for i := range clearCount {
			c.paramKeys[i] = ""
			c.paramValues[i] = ""
		}
		c.paramCount = 0
	}
	if c.Params != nil {
		clear(c.Params)
	}

	// Clear other fields
	c.version = ""
	c.routeTemplate = ""
	c.aborted = false
	c.index = -1

	// Mark as released BEFORE returning to pool (already done by Swap above)
	// This prevents use-after-release if someone still has a reference
	// Note: released flag was already set to true by Swap() above

	// Save paramCount before reset (needed for pool selection)
	paramCount := c.paramCount

	// Call reset() manually to clear all fields (including setting released=false for reuse)
	// This ensures the context is properly reset for the next request
	c.reset()

	// Return to appropriate pool based on original paramCount
	if c.router != nil && c.router.contextPool != nil {
		// Use router's context pool (preferred)
		// Set paramCount temporarily for pool selection
		c.paramCount = paramCount
		c.router.contextPool.Put(c)
	} else {
		// Fallback to global pool for static routes
		globalContextPool.Put(c)
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

	// Set metrics recorder for handler access to custom metrics
	if router.metrics != nil {
		c.metricsRecorder = router.metrics
	}
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

	// Set metrics recorder for handler access to custom metrics
	if router.metrics != nil {
		c.metricsRecorder = router.metrics
	}
}

// unsafeStringToBytes converts a string to a byte slice without allocation.
// This is safe for read-only operations (like writing to http.ResponseWriter)
// but MUST NOT be used if the resulting byte slice will be modified.
//
// This follows the same pattern used by fasthttp and other high-performance
// Go libraries for zero-copy string->bytes conversions.
//
// #nosec G103 - Intentional unsafe usage for performance optimization
func unsafeStringToBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
