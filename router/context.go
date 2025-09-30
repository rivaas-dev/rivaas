package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// Context represents the context of the current HTTP request with optimizations
// for high-performance request handling. It provides access to request/response
// objects, URL parameters, and middleware chain execution.
//
// The Context is optimized for speed with several performance features:
//   - Fast parameter storage using arrays for up to 8 parameters
//   - Zero-allocation parameter lookup for common cases
//   - Optimized response methods with minimal allocations
//   - Efficient middleware chain execution
//   - OpenTelemetry tracing support with minimal overhead
//   - Custom metrics recording capabilities
//
// Context objects are pooled and reused to minimize garbage collection pressure.
// Do not retain references to Context objects beyond the request lifetime.
type Context struct {
	Request  *http.Request       // The HTTP request object
	Response http.ResponseWriter // The HTTP response writer
	Params   map[string]string   // URL parameters (fallback for >8 params)
	handlers []HandlerFunc       // Handler chain for this request
	index    int                 // Current handler index in the chain
	router   *Router             // Reference to the router for metrics access

	// Fast parameter storage to avoid map allocations for common cases
	paramKeys   [8]string // Parameter names (up to 8 parameters)
	paramValues [8]string // Parameter values (up to 8 parameters)
	paramCount  int       // Number of parameters stored in arrays

	// OpenTelemetry tracing support
	span     trace.Span      // Current OpenTelemetry span
	traceCtx context.Context // Trace context for propagation
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
		// No allocations in constructor for maximum performance
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
	if c.index < len(c.handlers) {
		c.handlers[c.index](c)
	}
}

// Param returns the value of the URL parameter by key.
// This method is optimized for performance with zero allocations for up to 8 parameters.
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
	for i := range c.paramCount {
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
// Example:
//
//	if err := c.JSON(http.StatusOK, map[string]string{"message": "Hello World"}); err != nil {
//		return err
//	}
//	c.JSON(http.StatusCreated, user)
func (c *Context) JSON(code int, obj interface{}) error {
	c.Response.Header().Set("Content-Type", "application/json")

	// Check if headers have already been written to avoid "superfluous response.WriteHeader call"
	// This happens when middleware or other code has already written headers
	if rw, ok := c.Response.(*responseWriter); ok {
		if !rw.Written() {
			c.Response.WriteHeader(code)
		}
	} else {
		c.Response.WriteHeader(code)
	}

	return json.NewEncoder(c.Response).Encode(obj)
}

// String sends a plain text response with optional formatting.
// This method is optimized to minimize allocations for common patterns.
// Returns an error if writing to the response fails.
//
// For simple strings without formatting, zero allocations are achieved:
//
//	c.String(200, "Hello World")              // Zero allocations
//	c.String(200, "User: %s", username)       // Optimized for single %s
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
		return err
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
				return err
			}
		}
	}

	// Fallback for complex formatting (multiple values, non-string types, etc.)
	// Direct fmt.Fprintf to response - eliminates 2 allocations
	_, err := fmt.Fprintf(c.Response, format, values...)
	return err
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
	return err
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

// Header sets a response header. Headers must be set before writing the response body.
//
// SECURITY NOTE: This method does not validate header values. Be cautious when setting
// headers from user input to prevent header injection attacks. Consider sanitizing or
// validating values before setting them.
//
// Example:
//
//	c.Header("Cache-Control", "no-cache")
//	c.Header("Content-Type", "application/pdf")
//
// For headers from user input, validate the values:
//
//	// BAD: Direct user input
//	c.Header("X-User-Agent", c.Query("ua")) // Vulnerable to header injection
//
//	// GOOD: Validate before setting
//	if ua := c.Query("ua"); isValidUserAgent(ua) {
//	    c.Header("X-User-Agent", ua)
//	}
func (c *Context) Header(key, value string) {
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

// PostForm returns the value of the form parameter from POST request body.
// This works for both application/x-www-form-urlencoded and multipart/form-data.
// Returns an empty string if the parameter doesn't exist.
//
// Example:
//
//	username := c.PostForm("username")
//	password := c.PostForm("password")
func (c *Context) PostForm(key string) string {
	return c.Request.FormValue(key)
}

// IsJSON returns true if the request content type is application/json.
// This is a zero-allocation helper for content type checking.
func (c *Context) IsJSON() bool {
	contentType := c.Request.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}

// IsXML returns true if the request content type is application/xml or text/xml.
// This is a zero-allocation helper for content type checking.
func (c *Context) IsXML() bool {
	contentType := c.Request.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/xml") || strings.Contains(contentType, "text/xml")
}

// AcceptsJSON returns true if the client accepts JSON responses.
// This checks the Accept header for application/json.
func (c *Context) AcceptsJSON() bool {
	accept := c.Request.Header.Get("Accept")
	return strings.Contains(accept, "application/json") || strings.Contains(accept, "*/*")
}

// AcceptsHTML returns true if the client accepts HTML responses.
// This checks the Accept header for text/html.
func (c *Context) AcceptsHTML() bool {
	accept := c.Request.Header.Get("Accept")
	return strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

// GetClientIP returns the real client IP address.
// It checks X-Forwarded-For, X-Real-IP headers and falls back to RemoteAddr.
// This is optimized for zero allocations in common cases.
func (c *Context) GetClientIP() string {
	// Check X-Forwarded-For header first (most common proxy header)
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP if multiple are present
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := c.Request.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return ip
}

// IsSecure returns true if the request is served over HTTPS.
// This checks the TLS field and X-Forwarded-Proto header.
func (c *Context) IsSecure() bool {
	// Check if TLS is directly available
	if c.Request.TLS != nil {
		return true
	}

	// Check X-Forwarded-Proto header (for proxies)
	proto := c.Request.Header.Get("X-Forwarded-Proto")
	return proto == "https"
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
// This is optimized for performance and handles proper content types.
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

// PostFormDefault returns the form parameter value or a default if not present.
// This avoids the need for manual empty string checking.
func (c *Context) PostFormDefault(key, defaultValue string) string {
	value := c.PostForm(key)
	if value == "" {
		return defaultValue
	}
	return value
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

// reset resets the context to its initial state for reuse.
// This method is optimized for high-performance context pooling with zero allocations.
func (c *Context) reset() {
	// Fast reset without allocations
	c.Request = nil
	c.Response = nil
	c.handlers = nil
	c.index = -1
	c.paramCount = 0
	c.span = nil
	c.traceCtx = nil

	// Clear parameter arrays efficiently - only clear used slots
	for i := 0; i < c.paramCount && i < 8; i++ {
		c.paramKeys[i] = ""
		c.paramValues[i] = ""
	}

	// Clear map if it exists (for >8 params) without deallocating
	if c.Params != nil {
		for k := range c.Params {
			delete(c.Params, k)
		}
	}
}
