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

// Package binding provides request data binding for HTTP handlers.
//
// The binding package maps values from various sources (query parameters,
// form data, JSON bodies, headers, cookies, path parameters) into Go structs
// using struct tags. It supports nested structs, slices, maps, pointers,
// custom types, default values, and type conversion.
//
// Note: For validation (required fields, enum constraints, etc.), use the
// rivaas.dev/validation package separately after binding.
//
// # Quick Start
//
// The package provides both generic and non-generic APIs:
//
//	// Generic (preferred when type is known)
//	user, err := binding.JSON[CreateUserRequest](body)
//
//	// Non-generic (when type comes from variable)
//	var user CreateUserRequest
//	err := binding.JSONTo(body, &user)
//
// # Source-Specific Functions
//
// Each binding source has dedicated functions:
//
//	// Query parameters
//	params, err := binding.Query[ListParams](r.URL.Query())
//
//	// Path parameters
//	params, err := binding.Path[GetUserParams](pathParams)
//
//	// Form data
//	data, err := binding.Form[FormData](r.PostForm)
//
//	// HTTP headers
//	headers, err := binding.Header[RequestHeaders](r.Header)
//
//	// Cookies
//	session, err := binding.Cookie[SessionData](r.Cookies())
//
//	// Multipart form data (with file uploads)
//	r.ParseMultipartForm(32 << 20)
//	req, err := binding.Multipart[UploadRequest](r.MultipartForm)
//
//	// JSON body
//	user, err := binding.JSON[CreateUserRequest](body)
//
//	// XML body
//	user, err := binding.XML[CreateUserRequest](body)
//
// # Multi-Source Binding
//
// Bind from multiple sources using From* options:
//
//	req, err := binding.Bind[CreateOrderRequest](
//	    binding.FromPath(pathParams),
//	    binding.FromQuery(r.URL.Query()),
//	    binding.FromHeader(r.Header),
//	    binding.FromJSON(body),
//	)
//
// # Multipart Form Binding
//
// Bind multipart forms with file uploads directly to structs using the form tag.
// This enables handling file uploads and form values in a single struct:
//
//	type UploadRequest struct {
//	    Avatar      *File   `form:"avatar"`       // Single file
//	    Attachments []*File `form:"attachments"`  // Multiple files
//	    Title       string  `form:"title"`        // Regular form field
//	    Settings    Config  `form:"settings"`     // JSON string auto-parsed
//	}
//
//	r.ParseMultipartForm(32 << 20) // Parse with 32MB max memory
//	req, err := binding.Multipart[UploadRequest](r.MultipartForm)
//	if err != nil {
//	    return err
//	}
//
//	// Access file properties
//	fmt.Printf("File: %s (%d bytes, %s)\n",
//	    req.Avatar.Name, req.Avatar.Size, req.Avatar.ContentType)
//
//	// Save file to disk
//	req.Avatar.Save("./uploads/" + req.Avatar.Name)
//
//	// Read file into memory (for small files)
//	data, _ := req.Avatar.Bytes()
//
//	// Stream large files
//	reader, _ := req.Avatar.Open()
//	defer reader.Close()
//	io.Copy(destination, reader)
//
// File fields use the *File or []*File types and support the same struct tag syntax
// as other binding sources (defaults, aliases, etc.). The File type provides
// methods for reading, streaming, and saving uploaded files with built-in
// security (filename sanitization, path cleaning).
//
// ## JSON in Form Fields
//
// Multipart binding automatically detects and parses JSON strings in form fields:
//
//	type PrintSettings struct {
//	    Orientation string `json:"orientation"`
//	    ColorMode   string `json:"color_mode"`
//	}
//
//	type Request struct {
//	    Document *File         `form:"document"`
//	    Settings PrintSettings `form:"settings"` // JSON auto-parsed
//	}
//
//	// HTML form:
//	// <input type="file" name="document">
//	// <input type="hidden" name="settings" value='{"orientation":"landscape","color_mode":"color"}'>
//
// If the form field value starts with { or [ and ends with } or ], the binding
// automatically attempts JSON unmarshaling. If JSON parsing fails, it falls back
// to dot-notation parsing (settings.orientation=landscape).
//
// ## File Security
//
// The File type includes built-in security features:
//
//   - Filename sanitization: Removes path components (../, ..\, etc.)
//   - Path cleaning: Save() validates destination paths
//   - Content type detection: Extracts MIME type from headers
//
// Best practices for file handling:
//
//	// Generate unique filenames to prevent overwrites
//	uniqueName := uuid.New().String() + file.Ext()
//	file.Save("./uploads/" + uniqueName)
//
//	// Validate file size and type before processing
//	if file.Size > maxSize {
//	    return errors.New("file too large")
//	}
//	if !allowedTypes[file.ContentType] {
//	    return errors.New("unsupported file type")
//	}
//
// # Configuration
//
// Use functional options to customize binding behavior:
//
//	user, err := binding.JSON[User](body,
//	    binding.WithUnknownFields(binding.UnknownError),
//	    binding.WithMaxDepth(16),
//	)
//
// # Reusable Binder
//
// For shared configuration, create a Binder instance:
//
//	binder := binding.MustNew(
//	    binding.WithConverter[uuid.UUID](uuid.Parse),
//	    binding.WithTimeLayouts("2006-01-02", "01/02/2006"),
//	)
//
//	// Use across handlers
//	user, err := binder.JSON[CreateUserRequest](body)
//	params, err := binder.Query[ListParams](r.URL.Query())
//
// # Struct Tags
//
// The package uses struct tags to map values:
//
//	type Request struct {
//	    // Query parameters
//	    Page   int    `query:"page" default:"1"`
//	    Limit  int    `query:"limit" default:"20"`
//
//	    // Path parameters
//	    UserID string `path:"user_id"`
//
//	    // Headers
//	    Auth   string `header:"Authorization"`
//
//	    // JSON body fields
//	    Name   string `json:"name"`
//	    Email  string `json:"email"`
//
//	    // For validation, use the validation package
//	    Status string `json:"status" validate:"required,oneof=active pending disabled"`
//	}
//
// # Supported Tag Types
//
//   - query: URL query parameters (?name=value)
//   - path: URL path parameters (/users/:id)
//   - form: Form data (application/x-www-form-urlencoded)
//   - header: HTTP headers
//   - cookie: HTTP cookies
//   - json: JSON body fields
//   - xml: XML body fields
//
// # Additional Serialization Formats
//
// The following formats are available as sub-packages:
//
//   - rivaas.dev/binding/yaml: YAML support (gopkg.in/yaml.v3)
//   - rivaas.dev/binding/toml: TOML support (github.com/BurntSushi/toml)
//   - rivaas.dev/binding/msgpack: MessagePack support (github.com/vmihailenco/msgpack/v5)
//   - rivaas.dev/binding/proto: Protocol Buffers support (google.golang.org/protobuf)
//
// Example with YAML:
//
//	import "rivaas.dev/binding/yaml"
//
//	config, err := yaml.YAML[Config](body)
//
// Example with TOML:
//
//	import "rivaas.dev/binding/toml"
//
//	config, err := toml.TOML[Config](body)
//
// Example with MessagePack:
//
//	import "rivaas.dev/binding/msgpack"
//
//	msg, err := msgpack.MsgPack[Message](body)
//
// Example with Protocol Buffers:
//
//	import "rivaas.dev/binding/proto"
//
//	user, err := proto.Proto[*pb.User](body)
//
// # Special Tags
//
//   - default:"value": Default value when field is not present
//
// For validation constraints (required, enum, etc.), use the rivaas.dev/validation
// package with the `validate` struct tag.
//
// # Type Conversion
//
// Built-in support for common types:
//
//   - Primitives: string, int*, uint*, float*, bool
//   - Time: time.Time, time.Duration
//   - Network: net.IP, net.IPNet, url.URL
//   - Slices: []T for any supported T
//   - Maps: map[string]T for any supported T
//   - Pointers: *T for any supported T
//   - encoding.TextUnmarshaler implementations
//
// Register custom converters:
//
//	binding.MustNew(
//	    binding.WithConverter[uuid.UUID](uuid.Parse),
//	    binding.WithConverter[decimal.Decimal](decimal.NewFromString),
//	)
//
// # Common Converter Patterns
//
// The package provides factory functions for common converter patterns:
//
// ## Custom Time Layouts
//
// Use [TimeConverter] to parse times in non-standard formats:
//
//	binder := binding.MustNew(
//	    binding.WithConverter(binding.TimeConverter(
//	        "01/02/2006",        // US format
//	        "02/01/2006",        // European format
//	        "2006-01-02 15:04",  // Custom datetime
//	    )),
//	)
//
// ## Duration Aliases
//
// Use [DurationConverter] to provide user-friendly duration names:
//
//	binder := binding.MustNew(
//	    binding.WithConverter(binding.DurationConverter(map[string]time.Duration{
//	        "fast":    100 * time.Millisecond,
//	        "normal":  1 * time.Second,
//	        "slow":    5 * time.Second,
//	        "default": 30 * time.Second,
//	    })),
//	)
//
//	// Query: ?timeout=fast  -> 100ms
//	// Query: ?timeout=1h30m -> 1h30m (standard duration strings still work)
//
// ## Enum Validation
//
// Use [EnumConverter] to validate against allowed values:
//
//	type Status string
//	const (
//	    StatusActive   Status = "active"
//	    StatusPending  Status = "pending"
//	    StatusDisabled Status = "disabled"
//	)
//
//	binder := binding.MustNew(
//	    binding.WithConverter(binding.EnumConverter(
//	        StatusActive, StatusPending, StatusDisabled,
//	    )),
//	)
//
//	// Query: ?status=ACTIVE   -> StatusActive (case-insensitive)
//	// Query: ?status=unknown  -> error
//
// ## Custom Boolean Values
//
// Use [BoolConverter] to accept non-standard boolean representations:
//
//	binder := binding.MustNew(
//	    binding.WithConverter(binding.BoolConverter(
//	        []string{"enabled", "active", "on"},    // truthy values
//	        []string{"disabled", "inactive", "off"}, // falsy values
//	    )),
//	)
//
//	// Query: ?feature=enabled  -> true
//	// Query: ?feature=off      -> false
//
// ## Combining Multiple Converters
//
// You can use multiple converter factories together:
//
//	binder := binding.MustNew(
//	    binding.WithConverter(binding.TimeConverter("01/02/2006")),
//	    binding.WithConverter(binding.DurationConverter(map[string]time.Duration{
//	        "fast": 100 * time.Millisecond,
//	        "slow": 5 * time.Second,
//	    })),
//	    binding.WithConverter(binding.EnumConverter(StatusActive, StatusInactive)),
//	    binding.WithConverter(binding.BoolConverter(
//	        []string{"yes", "on"},
//	        []string{"no", "off"},
//	    )),
//	)
//
// ## Third-Party Type Examples
//
// For types from third-party packages, use [WithConverter]:
//
//	import (
//	    "github.com/google/uuid"
//	    "github.com/shopspring/decimal"
//	)
//
//	binder := binding.MustNew(
//	    binding.WithConverter[uuid.UUID](uuid.Parse),
//	    binding.WithConverter[decimal.Decimal](decimal.NewFromString),
//	)
//
// Or with a custom wrapper for error handling:
//
//	binder := binding.MustNew(
//	    binding.WithConverter[MyCustomType](func(s string) (MyCustomType, error) {
//	        // Custom parsing logic
//	        return parseMyCustomType(s)
//	    }),
//	)
//
// # Error Handling
//
// Errors provide detailed context:
//
//	user, err := binding.JSON[User](body)
//	if err != nil {
//	    var bindErr *binding.BindError
//	    if errors.As(err, &bindErr) {
//	        fmt.Printf("Field: %s, Source: %s, Value: %s\n",
//	            bindErr.Field, bindErr.Source, bindErr.Value)
//	    }
//	}
//
// Collect all errors instead of failing on first:
//
//	user, err := binding.JSON[User](body, binding.WithAllErrors())
//	if err != nil {
//	    var multi *binding.MultiError
//	    if errors.As(err, &multi) {
//	        for _, e := range multi.Errors {
//	            // Handle each error
//	        }
//	    }
//	}
//
// # Observability
//
// Add hooks for monitoring:
//
//	binder := binding.MustNew(
//	    binding.WithEvents(binding.Events{
//	        FieldBound: func(name, tag string) {
//	            log.Printf("Bound field %s from %s", name, tag)
//	        },
//	        Done: func(stats binding.Stats) {
//	            log.Printf("Bound %d fields", stats.FieldsBound)
//	        },
//	    }),
//	)
//
// # Security Limits
//
// Built-in limits prevent resource exhaustion:
//
//   - MaxDepth: Maximum struct nesting depth (default: 32)
//   - MaxSliceLen: Maximum slice elements (default: 10,000)
//   - MaxMapSize: Maximum map entries (default: 1,000)
//
// Configure limits:
//
//	binding.MustNew(
//	    binding.WithMaxDepth(16),
//	    binding.WithMaxSliceLen(1000),
//	    binding.WithMaxMapSize(500),
//	)
//
// # Configuration Options
//
// The package provides extensive configuration through functional options:
//
// ## Security Limits
//
//	WithMaxDepth(n int)        // Max struct nesting depth (default: 32)
//	WithMaxSliceLen(n int)     // Max slice elements (default: 10,000)
//	WithMaxMapSize(n int)      // Max map entries (default: 1,000)
//
// ## Unknown Fields
//
//	WithStrictJSON()  // Convenience: fail on unknown fields
//	WithUnknownFields(policy UnknownFieldPolicy)
//	  - UnknownIgnore: Ignore unknown fields (default)
//	  - UnknownWarn:   Log warnings via events
//	  - UnknownError:  Return error on unknown fields (same as WithStrictJSON)
//
// ## Slice Parsing
//
//	WithSliceMode(mode SliceParseMode)
//	  - SliceRepeat: Parse "tags=go&tags=rust" (default)
//	  - SliceCSV:    Parse "tags=go,rust,python"
//
// ## Time Formats
//
//	WithTimeLayouts(layouts ...string)  // Custom time.Time parsing formats
//	// Default layouts exported as DefaultTimeLayouts: RFC3339, RFC3339Nano, DateOnly, DateTime
//	// Extend defaults: WithTimeLayouts(append(DefaultTimeLayouts, "01/02/2006")...)
//
// ## Type Converters
//
//	WithConverter[T any](converter TypeConverter[T])
//	// Register custom type conversion function
//
// ## Error Handling
//
//	WithAllErrors()  // Collect all errors instead of failing on first
//
// ## Observability
//
//	WithEvents(events Events)  // Add hooks for monitoring
//	  - FieldBound:    Called when field is bound
//	  - UnknownField:  Called when unknown field detected
//	  - Done:          Called when binding completes
//
// ## Key Normalization
//
//	WithKeyNormalizer(normalizer KeyNormalizer)
//	// Custom key transformation for lookups
//
// # Advanced Tag Syntax
//
// ## Tag Aliases
//
// Provide multiple lookup names for a field:
//
//	type Request struct {
//	    UserID int `query:"user_id,id"` // Looks for "user_id" or "id"
//	}
//
// ## Nested Structs
//
// Use dot notation for nested fields:
//
//	type Address struct {
//	    Street string `query:"street"`
//	    City   string `query:"city"`
//	}
//
//	type User struct {
//	    Address Address `query:"address"`
//	}
//
//	// Query: ?address.street=123+Main&address.city=Boston
//
// ## Bracket Notation
//
// Arrays can use bracket notation:
//
//	type Request struct {
//	    Tags []string `query:"tags"`
//	}
//
//	// Both work: ?tags=go&tags=rust or ?tags[]=go&tags[]=rust
//
// # Streaming with io.Reader
//
// For large payloads, use Reader variants to avoid loading entire body into memory:
//
//	// JSON from reader
//	user, err := binding.JSONReader[User](r.Body)
//
//	// XML from reader
//	doc, err := binding.XMLReader[Document](r.Body)
//
// Reader variants are available for all body-based sources (JSON, XML, and sub-packages).
//
// # Performance Characteristics
//
// ## Caching
//
// Struct reflection information is cached automatically:
//
//   - First binding of a type: ~500ns overhead for reflection
//   - Subsequent bindings: ~50ns overhead (cache lookup)
//   - Cache is thread-safe and has no size limit
//   - Cache key includes both struct type and tag name
//
// ## Memory Allocation
//
//   - Query/Path/Form/Header/Cookie: Zero allocations for primitive types
//   - JSON/XML: Allocations depend on encoding/json and encoding/xml
//   - Nested structs: One allocation per nesting level
//   - Slices/Maps: Pre-allocated with capacity hints when possible
//
// ## Multi-Source Binding Precedence
//
// When using [Bind] with multiple sources, later sources override earlier ones:
//
//	req, err := binding.Bind[Request](
//	    binding.FromPath(pathParams),    // Applied first
//	    binding.FromQuery(r.URL.Query()), // Overrides path params
//	    binding.FromJSON(body),           // Overrides query params
//	)
//
// This allows for flexible request handling where body data takes precedence
// over URL parameters.
//
// # Custom ValueGetter
//
// For simple map-based sources, use the convenience helpers:
//
//	// Single-value map
//	getter := binding.MapGetter(map[string]string{"name": "Alice", "age": "30"})
//	result, err := binding.RawInto[User](getter, "custom")
//
//	// Multi-value map (for slices)
//	getter := binding.MultiMapGetter(map[string][]string{"tags": {"go", "rust"}})
//	result, err := binding.RawInto[User](getter, "custom")
//
// For more complex sources, implement the [ValueGetter] interface:
//
//	type CustomGetter struct {
//	    data map[string]string
//	}
//
//	func (g *CustomGetter) Get(key string) string {
//	    return g.data[key]
//	}
//
//	func (g *CustomGetter) GetAll(key string) []string {
//	    if val, ok := g.data[key]; ok {
//	        return []string{val}
//	    }
//	    return nil
//	}
//
//	func (g *CustomGetter) Has(key string) bool {
//	    _, ok := g.data[key]
//	    return ok
//	}
//
//	// Use with RawInto
//	result, err := binding.RawInto[MyStruct](&CustomGetter{data: myData}, "custom")
//
// Alternatively, use [GetterFunc] for a function-based adapter:
//
//	getter := binding.GetterFunc(func(key string) ([]string, bool) {
//	    if val, ok := myMap[key]; ok {
//	        return []string{val}, true
//	    }
//	    return nil, false
//	})
//
// # Integration Examples
//
// ## With net/http
//
//	func CreateUserHandler(w http.ResponseWriter, r *http.Request) {
//	    body, _ := io.ReadAll(r.Body)
//	    defer r.Body.Close()
//
//	    user, err := binding.JSON[CreateUserRequest](body)
//	    if err != nil {
//	        http.Error(w, err.Error(), http.StatusBadRequest)
//	        return
//	    }
//	    // Process user...
//	}
//
// ## With rivaas.dev/router
//
//	func CreateUserHandler(c *router.Context) {
//	    user, err := binding.JSON[CreateUserRequest](c.Body())
//	    if err != nil {
//	        c.Error(err, http.StatusBadRequest)
//	        return
//	    }
//	    c.JSON(http.StatusCreated, user)
//	}
//
// ## With rivaas.dev/app
//
//	func CreateUserHandler(c *app.Context) {
//	    var user CreateUserRequest
//	    if err := c.Bind(&user); err != nil {
//	        return // Error automatically handled
//	    }
//	    c.JSON(http.StatusCreated, user)
//	}
//
//	// Multipart file upload with app.Context
//	func UploadHandler(c *app.Context) {
//	    type Request struct {
//	        File  *binding.File `form:"file"`
//	        Title string         `form:"title"`
//	    }
//
//	    var req Request
//	    if err := c.Bind(&req); err != nil {
//	        return // Error automatically handled
//	    }
//
//	    req.File.Save("./uploads/" + req.File.Name)
//	    c.JSON(http.StatusOK, map[string]string{"uploaded": req.File.Name})
//	}
//
// # Best Practices
//
//   - Use generic API ([JSON], [Query], etc.) for compile-time type safety
//   - Create reusable [Binder] instances for shared configuration
//   - Set security limits ([WithMaxDepth], [WithMaxSliceLen], [WithMaxMapSize])
//   - Use Reader variants for large payloads (>1MB)
//   - Use rivaas.dev/validation package for validation (required, enum, etc.)
//   - Add observability hooks with [WithEvents] for monitoring
//   - Collect all errors with [WithAllErrors] for better UX
//
// # Error Types
//
// The package provides detailed error types for different failure scenarios:
//
//   - [BindError]: Field-level binding errors with context
//   - [UnknownFieldError]: Unknown fields in strict JSON mode
//   - [MultiError]: Multiple errors collected with [WithAllErrors]
//
// All error types implement standard error interfaces and integrate with
// rivaas.dev/errors for HTTP status code mapping.
package binding
