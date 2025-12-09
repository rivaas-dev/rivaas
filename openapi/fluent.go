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

package openapi

import (
	"reflect"

	"rivaas.dev/openapi/example"
)

// isZeroValue checks if a value is the zero value for its type.
// Used to avoid generating meaningless examples from empty structs.
func isZeroValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return true
	}
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return rv.IsNil()
	case reflect.Struct:
		for i := range rv.NumField() {
			field := rv.Field(i)
			// Skip unexported fields - they cannot be accessed via Interface()
			if !field.CanInterface() {
				continue
			}
			if !isZeroValue(field.Interface()) {
				return false
			}
		}

		return true
	default:
		return reflect.DeepEqual(rv.Interface(), reflect.Zero(rv.Type()).Interface())
	}
}

// Doc sets both the operation summary and description.
//
// This is a convenience method that sets both fields in a single call.
// The summary should be a brief one-line description, while the description
// can be longer and support Markdown formatting.
//
// Example:
//
//	app.GET("/users/:id", handler).
//	    Doc("Get user", "Retrieves a user by their unique identifier. Returns 404 if not found.")
func (rw *RouteWrapper) Doc(summary, description string) *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	rw.doc.Summary = summary
	rw.doc.Description = description

	return rw
}

// Summary sets the operation summary.
//
// The summary is a brief, one-line description of what the operation does.
// It appears in the operation list in Swagger UI.
//
// Example:
//
//	app.GET("/users", handler).Summary("List all users")
func (rw *RouteWrapper) Summary(s string) *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	rw.doc.Summary = s

	return rw
}

// Description sets the operation description.
//
// The description provides detailed information about the operation and
// supports Markdown formatting. It appears in the expanded operation view.
//
// Example:
//
//	app.POST("/users", handler).
//	    Description("Creates a new user account. Requires admin privileges.")
func (rw *RouteWrapper) Description(d string) *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	rw.doc.Description = d

	return rw
}

// Tags adds tags to the operation for grouping in Swagger UI.
//
// Tags help organize operations into logical groups. Operations with the same
// tag appear together in Swagger UI. Multiple tags can be added by calling
// this method multiple times or passing multiple arguments.
//
// Example:
//
//	app.GET("/users", handler).Tags("users", "management")
//	app.POST("/users", handler).Tags("users")
func (rw *RouteWrapper) Tags(tags ...string) *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	rw.doc.Tags = append(rw.doc.Tags, tags...)

	return rw
}

// OperationID sets a custom operation ID for this operation.
//
// Operation IDs are used by code generators and API client libraries. If not
// set, a semantic operation ID is automatically generated from the HTTP method
// and path (e.g., "getUserById" for GET /users/:id).
//
// Operation IDs must be unique across all operations. If a duplicate is detected,
// spec generation will return an error.
//
// Example:
//
//	app.GET("/users/:id", handler).OperationID("getUserById")
func (rw *RouteWrapper) OperationID(id string) *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	rw.doc.OperationID = id

	return rw
}

// Deprecated marks the operation as deprecated.
//
// Deprecated operations are visually distinguished in Swagger UI and should
// include information in the description about migration paths or alternatives.
//
// Example:
//
//	app.GET("/old-endpoint", handler).
//	    Deprecated().
//	    Description("Deprecated: Use /new-endpoint instead. Will be removed in v2.0.")
func (rw *RouteWrapper) Deprecated() *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	rw.doc.Deprecated = true

	return rw
}

// Consumes sets the content types that this operation accepts in request bodies.
//
// Default: ["application/json"]. Multiple content types can be specified.
// The first content type is used as the default for request body schemas.
//
// Example:
//
//	app.POST("/upload", handler).Consumes("multipart/form-data", "application/json")
func (rw *RouteWrapper) Consumes(ct ...string) *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	rw.doc.Consumes = ct

	return rw
}

// Produces sets the content types that this operation returns in responses.
//
// Default: ["application/json"]. Multiple content types can be specified.
// The first content type is used as the default for response schemas.
//
// Example:
//
//	app.GET("/export", handler).Produces("application/json", "text/csv")
func (rw *RouteWrapper) Produces(ct ...string) *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	rw.doc.Produces = ct

	return rw
}

// Bearer adds Bearer (JWT) authentication requirement to the operation.
//
// This is a convenience method that calls Security("bearerAuth"). The security
// scheme must be defined in the OpenAPI configuration using WithBearerAuth().
//
// Example:
//
//	app.GET("/protected", handler).Bearer()
func (rw *RouteWrapper) Bearer() *RouteWrapper {
	return rw.Security("bearerAuth")
}

// OAuth adds OAuth authentication requirement with optional scopes.
//
// This is a convenience method that calls Security() with the OAuth scheme name.
// The security scheme must be defined in the OpenAPI configuration.
//
// Example:
//
//	app.GET("/admin", handler).OAuth("oauth2", "admin", "read")
func (rw *RouteWrapper) OAuth(scheme string, scopes ...string) *RouteWrapper {
	return rw.Security(scheme, scopes...)
}

// Security adds a security requirement to the operation.
//
// The scheme name must match a security scheme defined in the OpenAPI
// configuration. For OAuth schemes, scopes can be specified. Multiple
// security requirements can be added by calling this method multiple times.
//
// Example:
//
//	app.GET("/protected", handler).Security("bearerAuth")
//	app.GET("/oauth", handler).Security("oauth2", "read", "write")
func (rw *RouteWrapper) Security(scheme string, scopes ...string) *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	rw.doc.Security = append(rw.doc.Security, SecurityReq{
		Scheme: scheme,
		Scopes: scopes,
	})

	return rw
}

// Request sets the request type and optionally provides examples.
//
// The schema is generated from req's type. Examples are handled as follows:
//   - No examples provided: req itself is used as the example (if non-zero)
//   - Examples provided: they become named examples in OpenAPI
//
// Example (simple):
//
//	Request(CreateUserRequest{Name: "John", Email: "john@example.com"})
//
// Example (named examples):
//
//	Request(CreateUserRequest{},
//		example.New("minimal", CreateUserRequest{Name: "John"}),
//		example.New("complete", CreateUserRequest{Name: "John", Email: "john@example.com"}),
//	)
func (rw *RouteWrapper) Request(req any, examples ...example.Example) *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	rw.doc.RequestType = reflect.TypeOf(req)
	if len(examples) == 0 {
		// No named examples: use req as single example (if non-zero)
		if !isZeroValue(req) {
			rw.doc.RequestExample = req
		} else {
			rw.doc.RequestExample = nil
		}
		rw.doc.RequestNamedExamples = nil
	} else {
		// Named examples provided
		rw.doc.RequestNamedExamples = examples
		rw.doc.RequestExample = nil
	}

	return rw
}

// Response sets the response schema and examples for a status code.
//
// The schema is generated from resp's type. Examples are handled as follows:
//   - No examples provided: resp itself is used as the example (if non-zero)
//   - Examples provided: they become named examples in OpenAPI
//
// Pass nil for resp for status codes that don't return a body (e.g., 204 No Content).
//
// Example (simple - 99% use case):
//
//	Response(200, UserResponse{ID: 123, Name: "John"})
//
// Example (named examples):
//
//	Response(200, UserResponse{},
//		example.New("success", UserResponse{ID: 123}),
//		example.New("admin", UserResponse{ID: 1, Role: "admin"}),
//	)
func (rw *RouteWrapper) Response(status int, resp any, examples ...example.Example) *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	// Handle nil response (e.g., 204 No Content)
	if resp == nil {
		rw.doc.ResponseTypes[status] = nil
		return rw
	}

	// 1. ALWAYS extract type from resp for schema generation
	rw.doc.ResponseTypes[status] = reflect.TypeOf(resp)

	// 2. Handle examples based on what was provided
	if len(examples) == 0 {
		// No named examples: use resp as single example (if non-zero)
		if !isZeroValue(resp) {
			rw.doc.ResponseExample[status] = resp
		} else {
			delete(rw.doc.ResponseExample, status)
		}

		// Clear any previous named examples
		delete(rw.doc.ResponseNamedExamples, status)
	} else {
		// Named examples provided: use "examples" (plural) map
		rw.doc.ResponseNamedExamples[status] = examples
		// Clear any previous single example
		delete(rw.doc.ResponseExample, status)
	}

	return rw
}

// ResponseExample sets an example value for a response status code.
//
// Examples help API consumers understand the expected response format.
// The example should match the response type set by Response().
//
// Example:
//
//	app.GET("/users/:id", handler).
//	    Response(200, UserResponse{}).
//	    ResponseExample(200, UserResponse{
//	        ID:    123,
//	        Name:  "John Doe",
//	        Email: "john@example.com",
//	    })
func (rw *RouteWrapper) ResponseExample(status int, ex any) *RouteWrapper {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw
	}

	// Set single example and clear any named examples for this status
	rw.doc.ResponseExample[status] = ex
	delete(rw.doc.ResponseNamedExamples, status)

	return rw
}
