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
	"regexp"

	"rivaas.dev/router/compiler"
)

// Handler is a type alias for handler functions.
// In practice, this will be router.HandlerFunc (func(*Context)).
// Using any here avoids the import cycle with the main router package.
type Handler = any

// DiagnosticKind categorizes diagnostic events.
type DiagnosticKind string

const (
	// DiagHighParamCount indicates a route has more than 8 parameters.
	DiagHighParamCount DiagnosticKind = "route_param_count_high"
)

// Registrar is the interface that Router implements to enable route registration.
// This interface is used by Route and Group to interact with the router without
// creating an import cycle.
//
// All methods in this interface are called at startup time during route registration,
// not in the hot path (ServeHTTP). Therefore, interface dispatch overhead is acceptable.
type Registrar interface {
	// IsFrozen returns true if routes cannot be modified.
	IsFrozen() bool

	// IsWarmedUp returns true if Warmup() has been called.
	IsWarmedUp() bool

	// AddPendingRoute adds a route to the pending routes list for deferred registration.
	AddPendingRoute(route *Route)

	// RegisterRouteNow registers a route immediately (used after warmup).
	RegisterRouteNow(route *Route)

	// GetGlobalMiddleware returns a copy of the router's global middleware.
	GetGlobalMiddleware() []Handler

	// RecordRouteRegistration records a route registration for metrics/diagnostics.
	RecordRouteRegistration(method, path string)

	// Emit emits a diagnostic event.
	Emit(kind DiagnosticKind, msg string, data map[string]any)

	// UpdateRouteInfo updates route info for introspection when constraints are added.
	UpdateRouteInfo(method, path, version string, update func(info *Info))

	// RegisterNamedRoute registers a named route for reverse routing.
	RegisterNamedRoute(name string, route *Route) error

	// GetRouteCompiler returns the route compiler (for compiled route matching).
	GetRouteCompiler() *compiler.RouteCompiler

	// UseCompiledRoutes returns whether compiled route matching is enabled.
	UseCompiledRoutes() bool

	// AddRouteToTree adds a route to the routing tree.
	AddRouteToTree(method, path string, handlers []Handler, constraints []Constraint)

	// AddVersionRoute adds a route to a version-specific tree.
	AddVersionRoute(version, method, path string, handlers []Handler, constraints []Constraint)

	// StoreRouteInfo stores route info for introspection.
	StoreRouteInfo(info Info)

	// AddRouteWithConstraints adds a route with support for parameter constraints.
	// This is used by Group to add routes.
	AddRouteWithConstraints(method, path string, handlers []Handler) *Route

	// CacheRouteHandlers caches handlers on a compiled route with proper type conversion.
	// This is called by Route.RegisterRoute() to cache handlers for fast lookup.
	CacheRouteHandlers(compiledRoute *compiler.CompiledRoute, handlers []Handler)
}

// CompilerHandlers converts handlers to compiler-compatible format.
func CompilerHandlers(handlers []Handler) []compiler.HandlerFunc {
	compilerHandlers := make([]compiler.HandlerFunc, 0, len(handlers))
	for _, h := range handlers {
		compilerHandlers = append(compilerHandlers, compiler.HandlerFunc(h))
	}

	return compilerHandlers
}

// CompilerConstraints converts constraints to compiler-compatible format.
func CompilerConstraints(constraints []Constraint) []compiler.RouteConstraint {
	if len(constraints) == 0 {
		return nil
	}
	compilerConstraints := make([]compiler.RouteConstraint, 0, len(constraints))
	for _, c := range constraints {
		compilerConstraints = append(compilerConstraints, compiler.RouteConstraint{
			Param:   c.Param,
			Pattern: c.Pattern,
		})
	}

	return compilerConstraints
}

// CacheHandlers is now handled by the Registrar interface.
// The router implementation caches handlers with the proper concrete type.

// ConstraintFromPattern creates a Constraint from a parameter name and regex pattern.
// Panics if the pattern is invalid (by design for early error detection).
func ConstraintFromPattern(param, pattern string) Constraint {
	regex, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		panic("Invalid regex pattern for parameter '" + param + "': " + err.Error())
	}

	return Constraint{
		Param:   param,
		Pattern: regex,
	}
}
