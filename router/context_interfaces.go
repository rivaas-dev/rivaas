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
// All response methods return errors explicitly, following Go's idiomatic error handling.
// Callers must check and handle errors appropriately.
//
// Example:
//
//	if err := c.JSON(200, user); err != nil {
//	    slog.ErrorContext(c.Request.Context(), "failed to write json", "err", err)
//	    return
//	}
type ResponseWriter interface {
	// Response methods (all return errors)
	JSON(code int, obj any) error
	IndentedJSON(code int, obj any) error
	PureJSON(code int, obj any) error
	SecureJSON(code int, obj any, prefix ...string) error
	ASCIIJSON(code int, obj any) error
	String(code int, value string) error
	Stringf(code int, format string, values ...any) error
	HTML(code int, html string) error
	YAML(code int, obj any) error
	Data(code int, contentType string, data []byte) error

	// Status and headers
	Status(code int)
	Header(key, value string)
	Redirect(code int, location string)
	NoContent()
	SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool)
}

// ContextReader combines ParameterReader with additional context reading methods.
// This interface extends ParameterReader with methods that read context-specific
// information like version, route pattern, and request metadata.
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
	// Returns an empty string if versioning is not used.
	Version() string

	// IsVersion checks if the current API version matches the specified version.
	IsVersion(version string) bool

	// RoutePattern returns the matched route pattern.
	// Example: "/users/:id" or "_not_found"
	RoutePattern() string
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
