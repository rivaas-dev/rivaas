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
	"fmt"
	"net/http"
	"reflect"

	"rivaas.dev/openapi/example"
	"rivaas.dev/openapi/internal/schema"
	"rivaas.dev/openapi/validate"
)

// RequestMetadata contains auto-discovered information about a request struct.
//
// This is a type alias for the internal schema.RequestMetadata type.
type RequestMetadata = schema.RequestMetadata

// ParamSpec describes a single parameter extracted from struct tags.
//
// This is a type alias for the internal schema.ParamSpec type.
type ParamSpec = schema.ParamSpec

// Operation represents an OpenAPI operation (HTTP method + path + metadata).
// Create operations using the HTTP method constructors: GET, POST, PUT, PATCH, DELETE, etc.
type Operation struct {
	Method string       // HTTP method (GET, POST, etc.)
	Path   string       // URL path with parameters (e.g. "/users/:id")
	doc    operationDoc // Operation documentation (private)
}

// OperationOption configures an OpenAPI operation.
// Use with HTTP method constructors like GET, POST, PUT, etc.
type OperationOption func(*operationDoc)

// operationDoc holds OpenAPI documentation for a single operation.
// This is private - users interact through Operation and OperationOption.
type operationDoc struct {
	Summary               string
	Description           string
	OperationID           string
	Tags                  []string
	Deprecated            bool
	Consumes              []string
	Produces              []string
	RequestType           reflect.Type
	RequestExample        any               // Single unnamed example
	RequestNamedExamples  []example.Example // Named examples
	ResponseTypes         map[int]reflect.Type
	ResponseExample       map[int]any               // Single unnamed example per status
	ResponseNamedExamples map[int][]example.Example // Named examples per status
	Security              []SecurityReq
	Extensions            map[string]any // Operation-level extensions (x-*)
}

// SecurityReq represents a security requirement for an operation.
type SecurityReq struct {
	Scheme string
	Scopes []string
}

// buildOperation creates an Operation from method, path, and options.
func buildOperation(method, path string, opts ...OperationOption) Operation {
	// Validate path format
	if err := validate.ValidatePath(path); err != nil {
		panic(fmt.Sprintf("invalid path '%s': %v", path, err))
	}

	doc := operationDoc{
		Consumes:              []string{"application/json"},
		Produces:              []string{"application/json"},
		ResponseTypes:         make(map[int]reflect.Type),
		ResponseExample:       make(map[int]any),
		ResponseNamedExamples: make(map[int][]example.Example),
	}
	for _, opt := range opts {
		opt(&doc)
	}
	return Operation{
		Method: method,
		Path:   path,
		doc:    doc,
	}
}

// GET creates an Operation for a GET request.
//
// Example:
//
//	openapi.GET("/users/:id",
//	    openapi.WithSummary("Get user"),
//	    openapi.WithResponse(200, User{}),
//	)
func GET(path string, opts ...OperationOption) Operation {
	return buildOperation(http.MethodGet, path, opts...)
}

// POST creates an Operation for a POST request.
//
// Example:
//
//	openapi.POST("/users",
//	    openapi.WithSummary("Create user"),
//	    openapi.WithRequest(CreateUserRequest{}),
//	    openapi.WithResponse(201, User{}),
//	)
func POST(path string, opts ...OperationOption) Operation {
	return buildOperation(http.MethodPost, path, opts...)
}

// PUT creates an Operation for a PUT request.
//
// Example:
//
//	openapi.PUT("/users/:id",
//	    openapi.WithSummary("Update user"),
//	    openapi.WithRequest(UpdateUserRequest{}),
//	    openapi.WithResponse(200, User{}),
//	)
func PUT(path string, opts ...OperationOption) Operation {
	return buildOperation(http.MethodPut, path, opts...)
}

// PATCH creates an Operation for a PATCH request.
//
// Example:
//
//	openapi.PATCH("/users/:id",
//	    openapi.WithSummary("Partially update user"),
//	    openapi.WithRequest(PatchUserRequest{}),
//	    openapi.WithResponse(200, User{}),
//	)
func PATCH(path string, opts ...OperationOption) Operation {
	return buildOperation(http.MethodPatch, path, opts...)
}

// DELETE creates an Operation for a DELETE request.
//
// Example:
//
//	openapi.DELETE("/users/:id",
//	    openapi.WithSummary("Delete user"),
//	    openapi.WithResponse(204, nil),
//	)
func DELETE(path string, opts ...OperationOption) Operation {
	return buildOperation(http.MethodDelete, path, opts...)
}

// HEAD creates an Operation for a HEAD request.
//
// Example:
//
//	openapi.HEAD("/users/:id",
//	    openapi.WithSummary("Check user exists"),
//	)
func HEAD(path string, opts ...OperationOption) Operation {
	return buildOperation(http.MethodHead, path, opts...)
}

// OPTIONS creates an Operation for an OPTIONS request.
//
// Example:
//
//	openapi.OPTIONS("/users",
//	    openapi.WithSummary("Get supported methods"),
//	)
func OPTIONS(path string, opts ...OperationOption) Operation {
	return buildOperation(http.MethodOptions, path, opts...)
}

// TRACE creates an Operation for a TRACE request.
//
// Example:
//
//	openapi.TRACE("/users/:id",
//	    openapi.WithSummary("Trace request"),
//	)
func TRACE(path string, opts ...OperationOption) Operation {
	return buildOperation(http.MethodTrace, path, opts...)
}

// Op creates an Operation with a custom HTTP method.
// Prefer using the method-specific constructors (GET, POST, etc.) when possible.
//
// Example:
//
//	openapi.Op("CUSTOM", "/resource",
//	    openapi.WithSummary("Custom operation"),
//	)
func Op(method, path string, opts ...OperationOption) Operation {
	return buildOperation(method, path, opts...)
}

// WithSummary sets the operation summary.
//
// Example:
//
//	openapi.GET("/users/:id",
//	    openapi.WithSummary("Get user by ID"),
//	)
func WithSummary(s string) OperationOption {
	return func(d *operationDoc) { d.Summary = s }
}

// WithDescription sets the operation description.
//
// Example:
//
//	openapi.GET("/users/:id",
//	    openapi.WithDescription("Retrieves a user by their unique identifier"),
//	)
func WithDescription(s string) OperationOption {
	return func(d *operationDoc) { d.Description = s }
}

// WithOperationID sets a custom operation ID.
//
// Example:
//
//	openapi.GET("/users/:id",
//	    openapi.WithOperationID("getUserById"),
//	)
func WithOperationID(id string) OperationOption {
	return func(d *operationDoc) { d.OperationID = id }
}

// WithRequest sets the request type and optionally provides examples.
//
// Example:
//
//	openapi.POST("/users",
//	    openapi.WithRequest(CreateUserRequest{}),
//	)
func WithRequest(req any, examples ...example.Example) OperationOption {
	return func(d *operationDoc) {
		d.RequestType = reflect.TypeOf(req)
		if len(examples) == 0 {
			if !isZeroValue(req) {
				d.RequestExample = req
			}
		} else {
			d.RequestNamedExamples = examples
		}
	}
}

// WithResponse sets the response schema and examples for a status code.
//
// Example:
//
//	openapi.GET("/users/:id",
//	    openapi.WithResponse(200, User{}),
//	    openapi.WithResponse(404, ErrorResponse{}),
//	)
func WithResponse(status int, resp any, examples ...example.Example) OperationOption {
	return func(d *operationDoc) {
		if resp == nil {
			d.ResponseTypes[status] = nil
			return
		}

		d.ResponseTypes[status] = reflect.TypeOf(resp)

		if len(examples) == 0 {
			if !isZeroValue(resp) {
				d.ResponseExample[status] = resp
			}
		} else {
			d.ResponseNamedExamples[status] = examples
		}
	}
}

// WithTags adds tags to the operation.
//
// Example:
//
//	openapi.GET("/users/:id",
//	    openapi.WithTags("users", "authentication"),
//	)
func WithTags(tags ...string) OperationOption {
	return func(d *operationDoc) { d.Tags = append(d.Tags, tags...) }
}

// WithSecurity adds a security requirement.
//
// Example:
//
//	openapi.GET("/users/:id",
//	    openapi.WithSecurity("bearerAuth"),
//	)
//
//	openapi.POST("/users",
//	    openapi.WithSecurity("oauth2", "read:users", "write:users"),
//	)
func WithSecurity(scheme string, scopes ...string) OperationOption {
	return func(d *operationDoc) {
		d.Security = append(d.Security, SecurityReq{
			Scheme: scheme,
			Scopes: scopes,
		})
	}
}

// WithDeprecated marks the operation as deprecated.
//
// Example:
//
//	openapi.GET("/old-endpoint",
//	    openapi.WithDeprecated(),
//	)
func WithDeprecated() OperationOption {
	return func(d *operationDoc) { d.Deprecated = true }
}

// WithConsumes sets the content types that this operation accepts.
//
// Example:
//
//	openapi.POST("/users",
//	    openapi.WithConsumes("application/xml", "application/json"),
//	)
func WithConsumes(contentTypes ...string) OperationOption {
	return func(d *operationDoc) { d.Consumes = contentTypes }
}

// WithProduces sets the content types that this operation returns.
//
// Example:
//
//	openapi.GET("/users/:id",
//	    openapi.WithProduces("application/xml", "application/json"),
//	)
func WithProduces(contentTypes ...string) OperationOption {
	return func(d *operationDoc) { d.Produces = contentTypes }
}

// WithOperationExtension adds a specification extension to the operation.
//
// Extension keys MUST start with "x-". In OpenAPI 3.1.x, keys starting with
// "x-oai-" or "x-oas-" are reserved for the OpenAPI Initiative.
//
// Example:
//
//	openapi.GET("/users/:id",
//	    openapi.WithOperationExtension("x-rate-limit", 100),
//	    openapi.WithOperationExtension("x-internal", true),
//	)
func WithOperationExtension(key string, value any) OperationOption {
	return func(d *operationDoc) {
		if d.Extensions == nil {
			d.Extensions = make(map[string]any)
		}
		d.Extensions[key] = value
	}
}

// isZeroValue checks if a value is the zero value for its type.
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
