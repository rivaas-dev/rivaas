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

package route

import (
	"fmt"
	"maps"
	"net/url"
	"strings"
	"sync"

	"rivaas.dev/router/compiler"
)

// Route represents a registered route with optional constraints.
// This provides a fluent interface for adding constraints and metadata.
//
// Routes use deferred registration - they are collected when created but only
// added to the routing tree during Warmup() or on first request. This allows
// constraints to be added via fluent API without re-registration issues.
type Route struct {
	registrar        Registrar
	version          string // API version (empty = standard tree, non-empty = version-specific tree)
	method           string
	path             string
	handlers         []Handler
	constraints      []Constraint               // Legacy regex-based constraints
	typedConstraints map[string]ParamConstraint // New typed constraints
	registered       bool                       // Whether route has been registered to a tree
	compiled         bool                       // Whether typed constraints have been compiled

	// Route metadata (immutable after registration)
	name           string          // Human-readable name for reverse routing
	description    string          // Optional description
	tags           []string        // Optional tags for categorization
	reversePattern *ReversePattern // Compiled pattern for URL building
	group          *Group          // Reference to group for name prefixing
	versionGroup   any             // Reference to version group for name prefixing (router.VersionGroup)

	mu sync.Mutex // Protects route modifications during constraint addition
}

// NewRoute creates a new Route with the given registrar, method, path, and handlers.
func NewRoute(registrar Registrar, version, method, path string, handlers []Handler) *Route {
	return &Route{
		registrar: registrar,
		version:   version,
		method:    method,
		path:      path,
		handlers:  handlers,
	}
}

// SetGroup sets the group reference for name prefixing.
func (r *Route) SetGroup(g *Group) {
	r.group = g
}

// SetVersionGroup sets the version group reference for name prefixing.
func (r *Route) SetVersionGroup(vg any) {
	r.versionGroup = vg
}

// GetVersionGroup returns the version group reference.
func (r *Route) GetVersionGroup() any {
	return r.versionGroup
}

// RegisterRoute adds the route to the appropriate radix tree with its constraints.
// This is called during Warmup() for deferred route registration.
// The route.version field determines which tree to use:
//   - Empty string: standard tree
//   - Non-empty: version-specific tree
//
// This method is thread-safe and uses a mutex to prevent double registration.
func (r *Route) RegisterRoute() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.registered {
		return // Already registered, skip
	}
	r.registered = true

	// Combine global middleware with route handlers
	// IMPORTANT: Create a new slice to avoid aliasing bugs with append
	globalMiddleware := r.registrar.GetGlobalMiddleware()
	allHandlers := make([]Handler, 0, len(globalMiddleware)+len(r.handlers))
	allHandlers = append(allHandlers, globalMiddleware...)
	allHandlers = append(allHandlers, r.handlers...)

	// Convert typed constraints to regex constraints for validation
	allConstraints := r.convertTypedConstraintsToRegex()
	allConstraints = append(allConstraints, r.constraints...)

	// Add route to appropriate tree based on version
	if r.version != "" {
		// Version-specific tree - do NOT add to global route compiler
		// Versioned routes use version-specific trees and caches
		r.registrar.AddVersionRoute(r.version, r.method, r.path, allHandlers, allConstraints)
	} else {
		// Standard tree - update compiler FIRST, then radix tree
		//
		// IMPORTANT: Order matters for constraint validation during re-registration.
		// When Where() is called after initial registration, we must update the
		// compiler before the radix tree to avoid a race condition where:
		// 1. Radix tree is updated with new constraints
		// 2. Compiler still has old route without constraints
		// 3. Request matches old route in compiler, bypassing constraint validation
		//
		// By updating compiler first:
		// - Compiler gets new constraints immediately
		// - During brief window before radix tree update, requests either:
		//   a) Match in compiler with new constraints (correct)
		//   b) Fall through to radix tree with old state (acceptable for brief window)

		// Compile route for matching (if enabled)
		if r.registrar.UseCompiledRoutes() && r.registrar.GetRouteCompiler() != nil {
			compilerConstraints := CompilerConstraints(allConstraints)
			compilerHandlers := CompilerHandlers(allHandlers)
			compiledRoute := compiler.CompileRoute(r.method, r.path, compilerHandlers, compilerConstraints)

			// Cache the converted handlers with proper type conversion
			r.registrar.CacheRouteHandlers(compiledRoute, allHandlers)

			// Remove any existing route then add new one
			// This ensures constraints are enforced before radix tree is updated
			routeCompiler := r.registrar.GetRouteCompiler()
			routeCompiler.RemoveRoute(r.method, r.path)
			routeCompiler.AddRoute(compiledRoute)
		}

		// Update radix tree (fallback path)
		r.registrar.AddRouteToTree(r.method, r.path, allHandlers, allConstraints)
	}
}

// Where adds a constraint to a route parameter using a regular expression.
// The constraint is pre-compiled for validation during routing.
// This method provides a fluent interface for building routes with validation.
//
// IMPORTANT: This method panics if the regex pattern is invalid. This is intentional
// for validation during application startup. Ensure patterns are tested.
//
// Common patterns:
//   - Numeric: `\d+` (one or more digits)
//   - Alpha: `[a-zA-Z]+` (letters only)
//   - UUID: `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`
//
// Example:
//
//	r.GET("/users/:id", getUserHandler).Where("id", `\d+`)
//	r.GET("/files/:filename", getFileHandler).Where("filename", `[a-zA-Z0-9.-]+`)
//
// The panic on invalid regex is by design for early error detection during development.
func (r *Route) Where(param, pattern string) *Route {
	// Pre-compile the regex pattern (panics on invalid pattern)
	constraint := ConstraintFromPattern(param, pattern)

	// Lock route for modifications
	r.mu.Lock()
	r.constraints = append(r.constraints, constraint)
	wasRegistered := r.registered
	r.registered = false // Allow re-registration with new constraints
	r.mu.Unlock()

	// Update RouteInfo with constraint for introspection
	r.registrar.UpdateRouteInfo(r.method, r.path, r.version, func(info *Info) {
		if info.Constraints == nil {
			info.Constraints = make(map[string]string)
		}
		info.Constraints[param] = pattern
	})

	// If route was already registered (after warmup), re-register with new constraints
	if wasRegistered {
		r.RegisterRoute()
	}

	return r
}

// WhereUUID adds a typed constraint that ensures the parameter is a valid UUID.
// This maps to OpenAPI schema type "string" with format "uuid".
//
// Example:
//
//	r.GET("/entities/:uuid", handler).WhereUUID("uuid")
func (r *Route) WhereUUID(name string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintUUID}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.RegisterRoute()
	}
	return r
}

// ensureTypedConstraints initializes the typed constraints map if needed.
func (r *Route) ensureTypedConstraints() {
	if r.typedConstraints == nil {
		r.typedConstraints = make(map[string]ParamConstraint)
	}
}

// WhereInt adds a typed constraint that ensures the parameter is an integer.
// This maps to OpenAPI schema type "integer" with format "int64".
//
// Example:
//
//	r.GET("/users/:id", handler).WhereInt("id")
func (r *Route) WhereInt(name string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintInt}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.RegisterRoute()
	}
	return r
}

// WhereFloat adds a typed constraint that ensures the parameter is a floating-point number.
// This maps to OpenAPI schema type "number" with format "double".
//
// Example:
//
//	r.GET("/prices/:amount", handler).WhereFloat("amount")
func (r *Route) WhereFloat(name string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintFloat}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.RegisterRoute()
	}
	return r
}

// WhereRegex adds a typed constraint with a custom regex pattern.
// This maps to OpenAPI schema type "string" with a pattern.
//
// Example:
//
//	r.GET("/files/:name", handler).WhereRegex("name", `[a-zA-Z0-9._-]+`)
func (r *Route) WhereRegex(name, pattern string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintRegex, Pattern: pattern}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.RegisterRoute()
	}
	return r
}

// WhereEnum adds a typed constraint that ensures the parameter matches one of the provided values.
// This maps to OpenAPI schema type "string" with an enum.
//
// Example:
//
//	r.GET("/status/:state", handler).WhereEnum("state", "active", "pending", "deleted")
func (r *Route) WhereEnum(name string, values ...string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{
		Kind: ConstraintEnum,
		Enum: append([]string(nil), values...),
	}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.RegisterRoute()
	}
	return r
}

// WhereDate adds a typed constraint that ensures the parameter is an RFC3339 full-date.
// This maps to OpenAPI schema type "string" with format "date".
//
// Example:
//
//	r.GET("/orders/:date", handler).WhereDate("date")
func (r *Route) WhereDate(name string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintDate}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.RegisterRoute()
	}
	return r
}

// WhereDateTime adds a typed constraint that ensures the parameter is an RFC3339 date-time.
// This maps to OpenAPI schema type "string" with format "date-time".
//
// Example:
//
//	r.GET("/events/:timestamp", handler).WhereDateTime("timestamp")
func (r *Route) WhereDateTime(name string) *Route {
	r.mu.Lock()
	r.ensureTypedConstraints()
	r.typedConstraints[name] = ParamConstraint{Kind: ConstraintDateTime}
	wasRegistered := r.registered
	r.registered = false
	r.mu.Unlock()

	if wasRegistered {
		r.RegisterRoute()
	}
	return r
}

// TypedConstraints returns a copy of the typed constraints map.
func (r *Route) TypedConstraints() map[string]ParamConstraint {
	if len(r.typedConstraints) == 0 {
		return nil
	}
	out := make(map[string]ParamConstraint, len(r.typedConstraints))
	maps.Copy(out, r.typedConstraints)
	return out
}

// compile compiles regex patterns in typed constraints (lazy compilation).
func (r *Route) compile() {
	if r.compiled {
		return
	}
	for k, pc := range r.typedConstraints {
		pc.Compile()
		r.typedConstraints[k] = pc
	}
	r.compiled = true
}

// convertTypedConstraintsToRegex converts typed constraints to regex-based Constraint
// for use with the existing validation system.
func (r *Route) convertTypedConstraintsToRegex() []Constraint {
	if len(r.typedConstraints) == 0 {
		return nil
	}

	r.compile()

	var regexConstraints []Constraint
	for name, pc := range r.typedConstraints {
		if constraint := pc.ToRegexConstraint(name); constraint != nil {
			regexConstraints = append(regexConstraints, *constraint)
		}
	}

	return regexConstraints
}

// SetName assigns a human-readable name to the route for reverse routing and introspection.
// Names must be globally unique. Panics if the router is frozen or if the name is already taken.
// Returns the route for method chaining.
//
// Example:
//
//	r.GET("/users/:id", getUserHandler).SetName("users.get")
//	r.POST("/users", createUserHandler).SetName("users.create")
func (r *Route) SetName(name string) *Route {
	if r.registrar.IsFrozen() {
		panic("cannot name routes after router is frozen")
	}

	// Auto-prefix with group name if in a group
	if r.group != nil && r.group.namePrefix != "" {
		name = r.group.namePrefix + name
	} else if r.versionGroup != nil {
		// Use type assertion to get namePrefix from VersionGroup
		// This avoids import cycle with main router package
		type namePrefixer interface {
			NamePrefix() string
		}
		if np, ok := r.versionGroup.(namePrefixer); ok {
			if prefix := np.NamePrefix(); prefix != "" {
				name = prefix + name
			}
		}
	}

	r.name = name
	if err := r.registrar.RegisterNamedRoute(name, r); err != nil {
		panic(err.Error())
	}

	return r
}

// SetDescription sets an optional description for the route.
// Useful for API documentation generation.
// Returns the route for method chaining.
//
// Example:
//
//	r.GET("/users/:id", getUserHandler).
//	    SetName("users.get").
//	    SetDescription("Retrieve a user by ID")
func (r *Route) SetDescription(desc string) *Route {
	r.description = desc
	return r
}

// SetTags adds categorization tags to the route.
// Useful for grouping routes in documentation and filtering.
// Returns the route for method chaining.
//
// Example:
//
//	r.GET("/users/:id", getUserHandler).
//	    SetName("users.get").
//	    SetTags("users", "public", "read")
func (r *Route) SetTags(tags ...string) *Route {
	r.tags = append(r.tags, tags...)
	return r
}

// Method returns the HTTP method for this route.
func (r *Route) Method() string {
	return r.method
}

// Path returns the route path pattern.
func (r *Route) Path() string {
	return r.path
}

// Name returns the route name (empty if not named).
// This follows Go naming conventions: getters don't use a Get prefix.
func (r *Route) Name() string {
	return r.name
}

// Description returns the route description (empty if not set).
func (r *Route) Description() string {
	return r.description
}

// Tags returns the route tags.
func (r *Route) Tags() []string {
	return r.tags
}

// Version returns the API version for this route (empty if not versioned).
func (r *Route) Version() string {
	return r.version
}

// Handlers returns the handler chain for this route.
func (r *Route) Handlers() []Handler {
	return r.handlers
}

// Constraints returns the regex-based constraints for this route.
func (r *Route) Constraints() []Constraint {
	return r.constraints
}

// ReversePattern returns the compiled reverse pattern for URL building.
func (r *Route) ReversePattern() *ReversePattern {
	return r.reversePattern
}

// SetReversePattern sets the compiled reverse pattern.
func (r *Route) SetReversePattern(p *ReversePattern) {
	r.reversePattern = p
}

// ReversePattern represents a compiled route pattern for URL building (reverse routing).
// It stores the positions of parameters to avoid string replacements.
type ReversePattern struct {
	Segments []Segment
}

// Segment represents a segment in a route path.
type Segment struct {
	Static bool   // true if static text, false if parameter
	Value  string // static text or parameter name
}

// ParseReversePattern parses a route path into segments for URL building.
// Example: "/users/:id/posts/:postId" -> [{static:"users"}, {param:"id"}, {static:"posts"}, {param:"postId"}]
func ParseReversePattern(path string) *ReversePattern {
	segments := make([]Segment, 0)
	trimmed := strings.Trim(path, "/")

	for part := range strings.SplitSeq(trimmed, "/") {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, ":") {
			// Parameter
			segments = append(segments, Segment{
				Static: false,
				Value:  part[1:], // Remove ":"
			})
		} else {
			// Static text
			segments = append(segments, Segment{
				Static: true,
				Value:  part,
			})
		}
	}

	return &ReversePattern{Segments: segments}
}

// BuildURL builds a URL from the reverse pattern and parameters.
func (p *ReversePattern) BuildURL(params map[string]string, query url.Values) (string, error) {
	var buf strings.Builder
	buf.WriteByte('/')

	for i, seg := range p.Segments {
		if i > 0 {
			buf.WriteByte('/')
		}

		if seg.Static {
			buf.WriteString(seg.Value)
		} else {
			val, ok := params[seg.Value]
			if !ok {
				return "", fmt.Errorf("missing required parameter: %s", seg.Value)
			}
			buf.WriteString(url.PathEscape(val))
		}
	}

	// Add query string if provided
	if len(query) > 0 {
		buf.WriteByte('?')
		buf.WriteString(query.Encode())
	}

	return buf.String(), nil
}
