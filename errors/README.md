# errors

Framework-agnostic error formatting for HTTP responses.

This package provides a clean, extensible way to format errors for HTTP APIs, supporting multiple response formats including RFC 9457 Problem Details, JSON:API, and simple JSON.

## Features

- **Multiple formats**: RFC 9457 Problem Details, JSON:API, Simple JSON
- **Content negotiation**: Choose format based on Accept header
- **Extensible**: Add custom formatters by implementing the `Formatter` interface
- **Framework-agnostic**: Works with any HTTP handler (net/http, Gin, Echo, etc.)
- **Type-safe**: Domain errors can implement optional interfaces to control formatting

## Quick Start

```go
import "rivaas.dev/errors"

// Create a formatter
formatter := errors.NewRFC9457("https://api.example.com/problems")

// Format an error
response := formatter.Format(req, err)

// Write response
w.WriteHeader(response.Status)
w.Header().Set("Content-Type", response.ContentType)
json.NewEncoder(w).Encode(response.Body)
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
    TypeResolver: func(err error) string {
        // Custom type resolution logic
        return "https://api.example.com/problems/custom-type"
    },
    StatusResolver: func(err error) int {
        // Custom status resolution logic
        return http.StatusBadRequest
    },
    ErrorIDGenerator: func() string {
        // Custom error ID generation
        return "custom-id-" + uuid.New().String()
    },
    DisableErrorID: false, // Set to true to disable error IDs
}
```

### JSON:API

JSON:API compliant error responses.

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

Simple, straightforward JSON error responses.

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
    
    w.WriteHeader(response.Status)
    w.Header().Set("Content-Type", response.ContentType)
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
    
    c.Response.WriteHeader(response.Status)
    c.Response.Header().Set("Content-Type", response.ContentType)
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
    return errors.Response{
        Status:      http.StatusBadRequest,
        ContentType: "application/json",
        Body:        map[string]string{"error": err.Error()},
    }
}
```

## Testing

The package includes comprehensive tests. Run them with:

```bash
go test ./errors/...
```

## License

See the main project license.
