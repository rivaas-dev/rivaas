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
// custom types, default values, enum validation, and type conversion.
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
// # Configuration
//
// Use functional options to customize binding behavior:
//
//	user, err := binding.JSON[User](body,
//	    binding.WithUnknownFields(binding.UnknownError),
//	    binding.WithRequired(),
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
//	    binding.WithRequired(),
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
//	    Name   string `json:"name" required:"true"`
//	    Email  string `json:"email" required:"true"`
//
//	    // Enum validation
//	    Status string `json:"status" enum:"active,pending,disabled"`
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
//   - enum:"a,b,c": Validate value is one of the allowed values
//   - required:"true": Field must be present (when WithRequired() is used)
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
// # Validation Integration
//
// Integrate external validators:
//
//	binder := binding.MustNew(
//	    binding.WithValidator(myValidator),
//	)
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
//	WithUnknownFields(policy UnknownFieldPolicy)
//	  - UnknownIgnore: Ignore unknown fields (default)
//	  - UnknownWarn:   Log warnings via events
//	  - UnknownError:  Return error on unknown fields
//
// ## Required Fields
//
//	WithRequired()  // Enforce required:"true" tags
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
//	// Default layouts: RFC3339, RFC3339Nano, DateOnly, DateTime
//
// ## Type Converters
//
//	WithConverter[T any](converter TypeConverter[T])
//	// Register custom type conversion function
//
// ## Validation
//
//	WithValidator(validator Validator)  // Integrate external validator
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
// Implement the [ValueGetter] interface for custom binding sources:
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
//	// Use with Raw/RawInto
//	result, err := binding.Raw[MyStruct](&CustomGetter{data: myData}, "custom")
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
// # Best Practices
//
//   - Use generic API ([JSON], [Query], etc.) for compile-time type safety
//   - Create reusable [Binder] instances for shared configuration
//   - Set security limits ([WithMaxDepth], [WithMaxSliceLen], [WithMaxMapSize])
//   - Use Reader variants for large payloads (>1MB)
//   - Validate enum values with enum:"value1,value2,value3" tags
//   - Integrate external validators with [WithValidator]
//   - Add observability hooks with [WithEvents] for monitoring
//   - Use [WithRequired] to enforce required:"true" tags
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
