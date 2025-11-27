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
	"maps"
	"reflect"
	"sync"

	"rivaas.dev/openapi/example"
	"rivaas.dev/openapi/internal/schema"
)

// RouteWrapper wraps route information with OpenAPI metadata.
//
// It provides a fluent API for adding OpenAPI documentation to routes.
//
// Concurrency: RouteWrapper is NOT safe for concurrent use during configuration.
// Each wrapper should be configured by a single goroutine. After calling [RouteWrapper.Freeze],
// the wrapper becomes read-only and is safe for concurrent reads. The wrapper
// uses internal locking to protect its mutable state.
type RouteWrapper struct {
	info   RouteInfo
	mu     sync.RWMutex
	doc    *RouteDoc
	frozen bool
}

// RouteDoc holds all OpenAPI metadata for a route.
//
// This struct is immutable after [RouteWrapper.Freeze] is called. It contains all
// information needed to generate an OpenAPI Operation specification.
type RouteDoc struct {
	Summary               string
	Description           string
	OperationID           string
	Tags                  []string
	Deprecated            bool
	Consumes              []string
	Produces              []string
	RequestType           reflect.Type
	RequestMetadata       *RequestMetadata
	RequestExample        any               // Single unnamed example (uses "example" field)
	RequestNamedExamples  []example.Example // Named examples (uses "examples" field)
	ResponseTypes         map[int]reflect.Type
	ResponseExample       map[int]any               // Single unnamed example per status
	ResponseNamedExamples map[int][]example.Example // Named examples per status
	Security              []SecurityReq
}

// SecurityReq represents a security requirement for an operation.
//
// It specifies which security scheme is required and, for OAuth schemes,
// which scopes are needed.
type SecurityReq struct {
	Scheme string
	Scopes []string
}

// NewRoute creates a new [RouteWrapper] for the given route information.
//
// The wrapper starts with default values and can be configured using
// the fluent API methods.
//
// Example:
//
//	route := openapi.NewRoute("GET", "/users/:id")
//	route.Doc("Get user", "Retrieves a user by ID")
func NewRoute(method, path string) *RouteWrapper {
	return NewRouteWithConstraints(method, path, nil)
}

// NewRouteWithConstraints creates a new [RouteWrapper] with typed path constraints.
//
// The constraints map parameter names to their type constraints, which are used
// to generate proper OpenAPI schema types for path parameters.
//
// Example:
//
//	constraints := map[string]PathConstraint{
//	    "id": {Kind: ConstraintInt},
//	}
//	route := openapi.NewRouteWithConstraints("GET", "/users/:id", constraints)
func NewRouteWithConstraints(method, path string, constraints map[string]PathConstraint) *RouteWrapper {
	return &RouteWrapper{
		info: RouteInfo{Method: method, Path: path, PathConstraints: constraints},
		doc: &RouteDoc{
			Tags:                  []string{},
			Consumes:              []string{"application/json"},
			Produces:              []string{"application/json"},
			RequestNamedExamples:  []example.Example{},
			ResponseTypes:         map[int]reflect.Type{},
			ResponseExample:       map[int]any{},
			ResponseNamedExamples: map[int][]example.Example{},
		},
	}
}

// Info returns the route information (method and path).
func (rw *RouteWrapper) Info() RouteInfo {
	return rw.info
}

// Freeze freezes the route metadata, making it immutable and thread-safe.
//
// This method:
//   - Performs automatic introspection of the request type if set
//   - Creates a deep copy of all metadata
//   - Marks the wrapper as frozen to prevent further modifications
//
// Freeze is idempotent - calling it multiple times returns the same
// frozen [RouteDoc]. This method is automatically called by [Manager.GenerateSpec]
// before spec generation.
//
// After freezing, all fluent API methods will have no effect.
//
// Returns the frozen [RouteDoc] for direct access.
func (rw *RouteWrapper) Freeze() *RouteDoc {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.frozen {
		return rw.doc
	}

	// Auto-introspect from request type
	if rw.doc.RequestType != nil {
		rw.doc.RequestMetadata = schema.IntrospectRequest(rw.doc.RequestType)
	}

	// Deep copy to freeze
	f := *rw.doc
	f.Tags = append([]string(nil), rw.doc.Tags...)
	f.Consumes = append([]string(nil), rw.doc.Consumes...)
	f.Produces = append([]string(nil), rw.doc.Produces...)
	f.Security = append([]SecurityReq(nil), rw.doc.Security...)
	f.RequestExample = rw.doc.RequestExample
	f.RequestNamedExamples = append([]example.Example(nil), rw.doc.RequestNamedExamples...)
	f.ResponseTypes = map[int]reflect.Type{}
	maps.Copy(f.ResponseTypes, rw.doc.ResponseTypes)
	f.ResponseExample = map[int]any{}
	maps.Copy(f.ResponseExample, rw.doc.ResponseExample)
	f.ResponseNamedExamples = map[int][]example.Example{}
	for k, v := range rw.doc.ResponseNamedExamples {
		f.ResponseNamedExamples[k] = append([]example.Example(nil), v...)
	}

	rw.doc = &f
	rw.frozen = true

	return &f
}

// GetFrozenDoc returns the frozen route documentation.
//
// Returns nil if the route has not been frozen yet. This method is
// thread-safe and can be called concurrently after [RouteWrapper.Freeze].
//
// This is primarily used internally during spec generation.
func (rw *RouteWrapper) GetFrozenDoc() *RouteDoc {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if !rw.frozen {
		return nil
	}

	return rw.doc
}
