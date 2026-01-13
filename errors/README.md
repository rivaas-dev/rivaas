# Errors

[![Go Reference](https://pkg.go.dev/badge/rivaas.dev/errors.svg)](https://pkg.go.dev/rivaas.dev/errors)
[![Go Report Card](https://goreportcard.com/badge/rivaas.dev/errors)](https://goreportcard.com/report/rivaas.dev/errors)
[![Coverage](https://codecov.io/gh/rivaas-dev/rivaas/branch/main/graph/badge.svg?component=module_errors)](https://codecov.io/gh/rivaas-dev/rivaas)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.25-blue)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](../LICENSE)

Framework-agnostic error formatting for HTTP responses.

This package provides a clean, extensible way to format errors for HTTP APIs, supporting multiple response formats including RFC 9457 Problem Details, JSON:API, and simple JSON.

## Features

- **Multiple formats**: RFC 9457 Problem Details, JSON:API, Simple JSON
- **Content negotiation**: Choose format based on Accept header
- **Extensible**: Add custom formatters by implementing the `Formatter` interface
- **Framework-agnostic**: Works with any HTTP handler (net/http, Gin, Echo, etc.)
- **Type-safe**: Domain errors can implement optional interfaces to control formatting

## Installation

```bash
go get rivaas.dev/errors
```

Requires Go 1.25+

## Quick Start

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    
    "rivaas.dev/errors"
)

func main() {
    http.HandleFunc("/api/users", handleGetUser)
    http.ListenAndServe(":8080", nil)
}

func handleGetUser(w http.ResponseWriter, req *http.Request) {
    // Your business logic
    user, err := getUser(req.URL.Query().Get("id"))
    if err != nil {
        // Create a formatter
        formatter := errors.NewRFC9457("https://api.example.com/problems")
        
        // Format the error
        response := formatter.Format(req, err)
        
        // Write response (set headers before status)
        w.Header().Set("Content-Type", response.ContentType)
        w.WriteHeader(response.Status)
        json.NewEncoder(w).Encode(response.Body)
        return
    }
    
    // Success response
    json.NewEncoder(w).Encode(user)
}

func getUser(id string) (*User, error) {
    if id == "" {
        return nil, fmt.Errorf("user ID is required")
    }
    // ... fetch user logic
    return &User{ID: id, Name: "John"}, nil
}

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}
```

## Formatters

### RFC 9457 Problem Details

RFC 9457 (formerly RFC 7807) provides a standardized way to represent errors in HTTP APIs.

```go
formatter := errors.NewRFC9457("https://api.example.com/problems")
response := formatter.Format(req, err)
```

**Response format:**

```json
{
  "type": "https://api.example.com/problems/validation_error",
  "title": "Bad Request",
  "status": 400,
  "detail": "Validation failed",
  "instance": "/api/users",
  "error_id": "err-abc123",
  "code": "validation_error",
  "errors": [...]
}
```

**Customization:**

```go
formatter := &errors.RFC9457{
    BaseURL: "https://api.example.com/problems",
    
    // TypeResolver maps errors to problem type URIs
    // If nil, uses ErrorCode interface or defaults to "about:blank"
    TypeResolver: func(err error) string {
        // Custom type resolution logic
        return "https://api.example.com/problems/custom-type"
    },
    
    // StatusResolver determines HTTP status from error
    // If nil, uses ErrorType interface or defaults to 500
    StatusResolver: func(err error) int {
        // Custom status resolution logic
        return http.StatusBadRequest
    },
    
    // ErrorIDGenerator generates unique IDs for error tracking
    // If nil, uses default cryptographically secure random ID
    ErrorIDGenerator: func() string {
        // Custom error ID generation
        return "custom-id-" + uuid.New().String()
    },
    
    // DisableErrorID disables automatic error ID generation
    DisableErrorID: false, // Set to true to disable error IDs
}
```

**Example with custom resolvers:**

```go
import (
    "errors"
    "net/http"
    "strings"
)

var (
    ErrNotFound      = errors.New("not found")
    ErrUnauthorized  = errors.New("unauthorized")
    ErrValidation    = errors.New("validation failed")
)

formatter := &errors.RFC9457{
    BaseURL: "https://api.example.com/problems",
    
    StatusResolver: func(err error) int {
        // Map specific errors to status codes
        switch {
        case errors.Is(err, ErrNotFound):
            return http.StatusNotFound
        case errors.Is(err, ErrUnauthorized):
            return http.StatusUnauthorized
        case errors.Is(err, ErrValidation):
            return http.StatusBadRequest
        default:
            return http.StatusInternalServerError
        }
    },
    
    TypeResolver: func(err error) string {
        // Map errors to problem type URIs
        errMsg := strings.ToLower(err.Error())
        switch {
        case strings.Contains(errMsg, "not found"):
            return "https://api.example.com/problems/not-found"
        case strings.Contains(errMsg, "unauthorized"):
            return "https://api.example.com/problems/unauthorized"
        case strings.Contains(errMsg, "validation"):
            return "https://api.example.com/problems/validation-error"
        default:
            return "about:blank"
        }
    },
}
```

### JSON:API

JSON:API compliant error responses. The formatter automatically generates unique error IDs for tracking and converts field paths to JSON Pointer format (`/data/attributes/...`).

```go
formatter := errors.NewJSONAPI()
response := formatter.Format(req, err)
```

**Response format:**

```json
{
  "errors": [
    {
      "id": "err-abc123",
      "status": "400",
      "code": "validation_error",
      "title": "Bad Request",
      "detail": "Validation failed",
      "source": {
        "pointer": "/data/attributes/email"
      }
    }
  ]
}
```

**Field Path Conversion:**

When errors implement `ErrorDetails` with field paths, they're automatically converted to JSON Pointer format:
- `"email"` → `"/data/attributes/email"`
- `"items.0.price"` → `"/data/attributes/items/0/price"`
- `"user.name"` → `"/data/attributes/user/name"`

**Customization:**

```go
formatter := &errors.JSONAPI{
    StatusResolver: func(err error) int {
        // Custom status resolution
        return http.StatusBadRequest
    },
}
```

### Simple JSON

Simple, straightforward JSON error responses. The `code` and `details` fields are optional and only included if the error implements the respective interfaces.

```go
formatter := errors.NewSimple()
response := formatter.Format(req, err)
```

**Response format:**

```json
{
  "error": "Something went wrong",
  "code": "internal_error",
  "details": {...}
}
```

**Field presence:**
- `error`: Always present (from `error.Error()`)
- `code`: Only if error implements `ErrorCode` interface
- `details`: Only if error implements `ErrorDetails` interface

**Customization:**

```go
formatter := &errors.Simple{
    StatusResolver: func(err error) int {
        // Custom status resolution
        return http.StatusBadRequest
    },
}
```

## Domain Error Interfaces

Your domain errors can implement optional interfaces to control how they're formatted:

### ErrorType

Control the HTTP status code:

```go
type NotFoundError struct {
    Resource string
}

func (e NotFoundError) Error() string {
    return fmt.Sprintf("%s not found", e.Resource)
}

func (e NotFoundError) HTTPStatus() int {
    return http.StatusNotFound
}
```

### ErrorCode

Provide a machine-readable error code:

```go
type ValidationError struct {
    Fields []FieldError
}

func (e ValidationError) Code() string {
    return "validation_error"
}
```

### ErrorDetails

Provide structured details (e.g., field-level validation errors):

```go
type ValidationError struct {
    Fields []FieldError
}

func (e ValidationError) Details() any {
    return e.Fields
}
```

## Content Negotiation

Use multiple formatters with content negotiation:

```go
formatters := map[string]errors.Formatter{
    "application/problem+json": errors.NewRFC9457("https://api.example.com/problems"),
    "application/vnd.api+json": errors.NewJSONAPI(),
    "application/json":         errors.NewSimple(),
}

// Select formatter based on Accept header
accept := req.Header.Get("Accept")
formatter := formatters[accept] // Add fallback logic as needed
response := formatter.Format(req, err)
```

## Integration Examples

### With net/http

```go
func errorHandler(w http.ResponseWriter, req *http.Request, err error) {
    formatter := errors.NewRFC9457("https://api.example.com/problems")
    response := formatter.Format(req, err)
    
    // Set headers before writing status
    w.Header().Set("Content-Type", response.ContentType)
    
    // Set any additional headers if present
    if response.Headers != nil {
        for key, values := range response.Headers {
            for _, value := range values {
                w.Header().Add(key, value)
            }
        }
    }
    
    w.WriteHeader(response.Status)
    json.NewEncoder(w).Encode(response.Body)
}
```

### With Custom Framework

```go
type MyContext struct {
    Request  *http.Request
    Response http.ResponseWriter
}

func (c *MyContext) Error(err error) {
    formatter := errors.NewRFC9457("https://api.example.com/problems")
    response := formatter.Format(c.Request, err)
    
    // Set headers before status
    c.Response.Header().Set("Content-Type", response.ContentType)
    c.Response.WriteHeader(response.Status)
    json.NewEncoder(c.Response).Encode(response.Body)
}
```

## Custom Formatters

Create your own formatter by implementing the `Formatter` interface:

```go
type CustomFormatter struct {
    // Your configuration
}

func (f *CustomFormatter) Format(req *http.Request, err error) errors.Response {
    // Your formatting logic
    headers := make(http.Header)
    headers.Set("X-Error-ID", generateID())
    headers.Set("X-Request-ID", req.Header.Get("X-Request-ID"))
    
    return errors.Response{
        Status:      http.StatusBadRequest,
        ContentType: "application/json",
        Body:        map[string]string{"error": err.Error()},
        Headers:     headers, // Optional: additional headers
    }
}
```

### Response Structure

The `Response` struct returned by formatters contains:

- **Status** (int): HTTP status code to return
- **ContentType** (string): Content-Type header value
- **Body** (any): Response body to be JSON-encoded
- **Headers** (http.Header): Optional additional headers to set

Example of using all fields:

```go
response := formatter.Format(req, err)

// Set content type
w.Header().Set("Content-Type", response.ContentType)

// Set any additional headers
if response.Headers != nil {
    for key, values := range response.Headers {
        for _, value := range values {
            w.Header().Add(key, value)
        }
    }
}

// Write status and body
w.WriteHeader(response.Status)
json.NewEncoder(w).Encode(response.Body)
```

## Testing

The package includes comprehensive tests. Run them with:

```bash
go test ./errors/...
```

## API Reference

For detailed API documentation, see [pkg.go.dev/rivaas.dev/errors](https://pkg.go.dev/rivaas.dev/errors).

## Contributing

Contributions are welcome! Please see the [main repository](../) for contribution guidelines.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.

---

Part of the [Rivaas](https://github.com/rivaas-dev/rivaas) web framework ecosystem.
