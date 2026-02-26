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

import (
	"net/http"
	"sync"
	"sync/atomic"
	"unsafe"

	"rivaas.dev/router/route"
)

// methodTrees holds per-method route tree roots for O(1) lookup via switch.
// Used instead of map[string]*node to avoid string hashing in the hot path.
type methodTrees struct {
	get     *node
	post    *node
	put     *node
	delete  *node
	patch   *node
	head    *node
	options *node
}

// getTree returns the tree for the given HTTP method, or nil.
func (m *methodTrees) getTree(method string) *node {
	switch method {
	case http.MethodGet:
		return m.get
	case http.MethodPost:
		return m.post
	case http.MethodPut:
		return m.put
	case http.MethodDelete:
		return m.delete
	case http.MethodPatch:
		return m.patch
	case http.MethodHead:
		return m.head
	case http.MethodOptions:
		return m.options
	default:
		return nil
	}
}

// setTree sets the tree for the given HTTP method.
func (m *methodTrees) setTree(method string, n *node) {
	switch method {
	case http.MethodGet:
		m.get = n
	case http.MethodPost:
		m.post = n
	case http.MethodPut:
		m.put = n
	case http.MethodDelete:
		m.delete = n
	case http.MethodPatch:
		m.patch = n
	case http.MethodHead:
		m.head = n
	case http.MethodOptions:
		m.options = n
	}
}

// iterate calls fn for each non-nil method tree.
func (m *methodTrees) iterate(fn func(method string, tree *node)) {
	if m.get != nil {
		fn(http.MethodGet, m.get)
	}
	if m.post != nil {
		fn(http.MethodPost, m.post)
	}
	if m.put != nil {
		fn(http.MethodPut, m.put)
	}
	if m.delete != nil {
		fn(http.MethodDelete, m.delete)
	}
	if m.patch != nil {
		fn(http.MethodPatch, m.patch)
	}
	if m.head != nil {
		fn(http.MethodHead, m.head)
	}
	if m.options != nil {
		fn(http.MethodOptions, m.options)
	}
}

// copy returns a new methodTrees with the same pointers (shallow copy for copy-on-write).
func (m *methodTrees) copy() *methodTrees {
	return &methodTrees{
		get: m.get, post: m.post, put: m.put, delete: m.delete,
		patch: m.patch, head: m.head, options: m.options,
	}
}

// atomicRouteTree represents a route tree with thread-safe operations.
// This structure enables concurrent reads and writes.
//
// SAFETY REQUIREMENTS:
//   - Requires 64-bit architecture for pointer operations
//   - Pointer must be properly aligned (guaranteed by Go runtime)
//   - Copy-on-write ensures readers never see partial updates
//
// FIELD ORDER REQUIREMENTS:
//   - `trees` MUST be the first field (offset 0) to guarantee 8-byte alignment
//   - `version` MUST follow immediately after for 8-byte alignment
//   - DO NOT reorder these fields or insert fields between them
//   - Operations on uint64/unsafe.Pointer require 8-byte alignment
//
// Platform Support:
//   - amd64: ✓ Fully supported
//   - arm64: ✓ Fully supported
//   - 386:   ✗ Not supported (32-bit)
//   - arm:   ✗ Not supported (32-bit)
//
// Alignment is verified at runtime in init() - the program will panic if misaligned.
type atomicRouteTree struct {
	// trees is a pointer to the current methodTrees (per-method roots)
	// This allows thread-safe reads and updates during route registration
	// WARNING: Must only be accessed via atomic operations (Load/Store/CompareAndSwap)
	// CRITICAL: Must be first field for 8-byte alignment (verified in init())
	trees unsafe.Pointer // *methodTrees

	// version is incremented on each tree update
	// CRITICAL: Must immediately follow trees for 8-byte alignment (verified in init())
	version uint64

	// routes is protected by a separate mutex for introspection (low-frequency access)
	routes      []route.Info
	routesMutex sync.RWMutex
}

func init() {
	// Runtime safety check: Verify platform support for atomic pointer operations
	// This ensures the router only runs on supported 64-bit architectures
	if unsafe.Sizeof(unsafe.Pointer(nil)) != 8 {
		panic("router: requires 64-bit architecture for atomic pointer operations (unsafe.Pointer must be 8 bytes)")
	}

	// Verify atomic field alignment at runtime
	// On 64-bit systems, atomic operations on uint64 and unsafe.Pointer require 8-byte alignment.
	// The Go compiler guarantees this for the first field and for fields following 8-byte aligned fields.
	// This check ensures our struct layout remains correct even if refactored.
	var tree atomicRouteTree
	treesOffset := unsafe.Offsetof(tree.trees)
	versionOffset := unsafe.Offsetof(tree.version)

	if treesOffset != 0 {
		panic("router: atomicRouteTree.trees must be first field for proper atomic alignment")
	}
	if versionOffset%8 != 0 {
		panic("router: atomicRouteTree.version is not 8-byte aligned (misaligned atomic operations will panic on some architectures)")
	}

	// Verify atomicVersionTrees alignment
	var vt atomicVersionTrees
	vtTreesOffset := unsafe.Offsetof(vt.trees)
	if vtTreesOffset != 0 {
		panic("router: atomicVersionTrees.trees must be first field for proper atomic alignment")
	}
}

// getTreeForMethodDirect atomically gets the tree for a specific HTTP method without copying.
// Uses a switch on method string to avoid map hashing in the hot path.
func (r *Router) getTreeForMethodDirect(method string) *node {
	treesPtr := atomic.LoadPointer(&r.routeTree.trees)
	trees := (*methodTrees)(treesPtr)
	if trees == nil {
		return nil
	}
	return trees.getTree(method)
}

// loadTrees atomically loads the method trees.
// Returns nil if trees haven't been initialized.
func (rt *atomicRouteTree) loadTrees() *methodTrees {
	treesPtr := atomic.LoadPointer(&rt.trees)
	if treesPtr == nil {
		return nil
	}
	return (*methodTrees)(treesPtr)
}

// GET adds a route that matches GET requests to the specified path.
// The path can contain parameters using the :param syntax.
// Returns a Route object for adding constraints and metadata.
//
// Example:
//
//	r.GET("/users/:id", getUserHandler)
//	r.GET("/health", healthCheckHandler)
//	r.GET("/users/:id", getUserHandler).Where("id", `\d+`) // With constraint
func (r *Router) GET(path string, handlers ...HandlerFunc) *route.Route {
	return r.addRoute(http.MethodGet, path, handlers)
}

// POST adds a route that matches POST requests to the specified path.
// Commonly used for creating resources and handling form submissions.
//
// Example:
//
//	r.POST("/users", createUserHandler)
//	r.POST("/login", loginHandler)
func (r *Router) POST(path string, handlers ...HandlerFunc) *route.Route {
	return r.addRoute(http.MethodPost, path, handlers)
}

// PUT adds a route that matches PUT requests to the specified path.
// Typically used for updating or replacing entire resources.
//
// Example:
//
//	r.PUT("/users/:id", updateUserHandler)
func (r *Router) PUT(path string, handlers ...HandlerFunc) *route.Route {
	return r.addRoute(http.MethodPut, path, handlers)
}

// DELETE adds a route that matches DELETE requests to the specified path.
// Used for removing resources from the server.
//
// Example:
//
//	r.DELETE("/users/:id", deleteUserHandler)
func (r *Router) DELETE(path string, handlers ...HandlerFunc) *route.Route {
	return r.addRoute(http.MethodDelete, path, handlers)
}

// PATCH adds a route that matches PATCH requests to the specified path.
// Used for partial updates to existing resources.
//
// Example:
//
//	r.PATCH("/users/:id", patchUserHandler)
func (r *Router) PATCH(path string, handlers ...HandlerFunc) *route.Route {
	return r.addRoute(http.MethodPatch, path, handlers)
}

// OPTIONS adds a route that matches OPTIONS requests to the specified path.
// Commonly used for CORS preflight requests and API discovery.
//
// Example:
//
//	r.OPTIONS("/api/*", corsHandler)
func (r *Router) OPTIONS(path string, handlers ...HandlerFunc) *route.Route {
	return r.addRoute(http.MethodOptions, path, handlers)
}

// HEAD adds a route that matches HEAD requests to the specified path.
// HEAD requests are like GET requests but return only headers without the response body.
//
// Example:
//
//	r.HEAD("/users/:id", checkUserExistsHandler)
func (r *Router) HEAD(path string, handlers ...HandlerFunc) *route.Route {
	return r.addRoute(http.MethodHead, path, handlers)
}

// addRoute adds a route with support for parameter constraints.
// Returns a Route object that can be used to add constraints and metadata.
//
// Routes use deferred registration - they are added to pendingRoutes and only
// registered to the routing tree during Warmup() or on first request. This allows
// the fluent Where* API to work correctly without re-registration issues.
func (r *Router) addRoute(method, path string, handlers []HandlerFunc) *route.Route {
	// Convert handlers to route.Handler
	routeHandlers := make([]route.Handler, 0, len(handlers))
	for _, h := range handlers {
		routeHandlers = append(routeHandlers, h)
	}

	return r.AddRouteWithConstraints(method, path, routeHandlers)
}
