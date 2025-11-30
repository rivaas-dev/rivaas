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
package binding
