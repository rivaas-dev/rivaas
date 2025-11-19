package router

// ParameterReader defines the interface for reading request parameters,
// query strings, form values, cookies, and other request data.
//
// This interface enables:
//   - Easier testing by allowing mock implementations
//   - Clearer separation of concerns
//   - Composition with other interfaces
//
// Example usage:
//
//	func processRequest(reader ParameterReader) {
//	    userID := reader.Param("id")
//	    page := reader.Query("page")
//	}
type ParameterReader interface {
	// Param returns the value of the URL path parameter by key.
	// Returns empty string if the parameter is not found.
	//
	// Example:
	//   Route: /users/:id
	//   Request: /users/123
	//   c.Param("id") // Returns "123"
	Param(key string) string

	// Query returns the value of the URL query parameter by key.
	// Returns empty string if the parameter is not found.
	// For parameters with multiple values, returns the last value.
	//
	// Example:
	//   Request: /search?q=golang&page=2
	//   c.Query("q")    // Returns "golang"
	//   c.Query("page") // Returns "2"
	Query(key string) string

	// QueryDefault returns the value of the URL query parameter by key,
	// or the default value if the parameter is not found.
	//
	// Example:
	//   c.QueryDefault("page", "1") // Returns "1" if "page" is not in query
	QueryDefault(key, defaultValue string) string

	// FormValue returns the value of the form field by key.
	// Handles both application/x-www-form-urlencoded and multipart/form-data.
	// Returns empty string if the field is not found.
	FormValue(key string) string

	// FormValueDefault returns the value of the form field by key,
	// or the default value if the field is not found.
	FormValueDefault(key, defaultValue string) string

	// AllParams returns all URL path parameters as a map.
	// Returns a new map (copy) to prevent external modification.
	AllParams() map[string]string

	// AllQueries returns all query parameters as a map.
	// For parameters with multiple values, returns the last value.
	AllQueries() map[string]string

	// GetCookie returns the value of the named cookie.
	// Returns an error if the cookie is not found.
	GetCookie(name string) (string, error)
}

// ResponseWriter defines the interface for writing HTTP responses.
//
// This interface enables:
//   - Easier testing by allowing mock implementations
//   - Clearer separation of concerns
//   - Composition with other interfaces
//
// Example usage:
//
//	func sendResponse(writer ResponseWriter) error {
//	    return writer.JSON(http.StatusOK, map[string]string{"status": "ok"})
//	}
type ResponseWriter interface {
	// JSON sends a JSON response with the specified status code.
	// Automatically sets Content-Type header to "application/json; charset=utf-8".
	//
	// Example:
	//   c.JSON(http.StatusOK, map[string]string{"message": "success"})
	JSON(code int, obj any) error

	// IndentedJSON sends a JSON response with indentation for readability.
	// Slower than JSON() but useful for debugging and development.
	//
	// Example:
	//   c.IndentedJSON(200, user)
	IndentedJSON(code int, obj any) error

	// PureJSON sends a JSON response without HTML escaping.
	// Useful when JSON contains HTML content that should not be escaped.
	//
	// Example:
	//   c.PureJSON(200, map[string]string{"html": "<div>content</div>"})
	PureJSON(code int, obj any) error

	// SecureJSON sends a JSON response with a security prefix to prevent JSON hijacking.
	// Default prefix: "while(1);"
	//
	// Example:
	//   c.SecureJSON(200, data)
	SecureJSON(code int, obj any, prefix ...string) error

	// ASCIIJSON sends a JSON response with all non-ASCII characters escaped to \uXXXX.
	// Useful for legacy systems requiring pure ASCII.
	//
	// Example:
	//   c.ASCIIJSON(200, map[string]string{"message": "Hello 世界"})
	ASCIIJSON(code int, obj any) error

	// String sends a plain text response with the specified status code.
	// Supports format strings similar to fmt.Printf.
	//
	// Example:
	//   c.String(http.StatusOK, "Hello, %s", name)
	//   c.String(http.StatusOK, "Simple message")
	String(code int, format string, values ...any) error

	// HTML sends an HTML response with the specified status code.
	//
	// Example:
	//   c.HTML(http.StatusOK, "<h1>Welcome</h1>")
	HTML(code int, html string) error

	// YAML sends a YAML response with the specified status code.
	//
	// Example:
	//   c.YAML(200, config)
	YAML(code int, obj any) error

	// Data sends raw data with the specified content type and status code.
	//
	// Example:
	//   c.Data(200, "image/png", imageBytes)
	Data(code int, contentType string, data []byte) error

	// Status sets the HTTP status code for the response.
	// Should be called before writing any response body.
	//
	// Example:
	//   c.Status(http.StatusNoContent) // No Content
	Status(code int)

	// Header sets a response header with automatic security sanitization.
	// Headers must be set before writing the response body.
	//
	// Example:
	//   c.Header("Cache-Control", "no-cache")
	Header(key, value string)

	// Redirect sends a redirect response to the specified location.
	//
	// Example:
	//   c.Redirect(301, "https://example.com/new-location")
	Redirect(code int, location string)

	// NoContent sends a 204 No Content response.
	//
	// Example:
	//   c.NoContent()
	NoContent()

	// SetCookie sets a cookie in the response.
	//
	// Example:
	//   c.SetCookie("session", "abc123", 3600, "/", "example.com", true, true)
	SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool)
}

// ContextReader combines ParameterReader with additional context reading methods.
// This interface extends ParameterReader with methods that read context-specific
// information like version, route template, and request metadata.
//
// Example usage:
//
//	func processRequest(reader ContextReader) {
//	    version := reader.Version()
//	    if reader.IsVersion("v2") {
//	        // Handle v2 API
//	    }
//	}
type ContextReader interface {
	ParameterReader

	// Version returns the current API version (e.g., "v1", "v2").
	// Returns empty string if versioning is not used.
	Version() string

	// IsVersion checks if the current API version matches the specified version.
	IsVersion(version string) bool

	// RouteTemplate returns the matched route template.
	// Example: "/users/:id" or "_not_found"
	RouteTemplate() string
}

// ContextWriter combines ResponseWriter with additional context writing methods.
// This interface extends ResponseWriter with methods that write context-specific
// responses.
type ContextWriter interface {
	ResponseWriter
}

// Ensure Context implements all interfaces at compile time.
var (
	_ ParameterReader = (*Context)(nil)
	_ ResponseWriter  = (*Context)(nil)
	_ ContextReader   = (*Context)(nil)
	_ ContextWriter   = (*Context)(nil)
)
