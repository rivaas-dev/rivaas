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

import "maps"

// MountConfig holds configuration for a mounted subrouter.
type MountConfig struct {
	InheritMiddleware bool
	ExtraMiddleware   []Handler
	NamePrefix        string
	NotFoundHandler   Handler
}

// MountOption configures how a subrouter is mounted.
type MountOption func(*MountConfig)

// InheritMiddleware makes the subrouter inherit parent router's global middleware.
// Parent middleware runs before subrouter middleware.
func InheritMiddleware() MountOption {
	return func(cfg *MountConfig) {
		cfg.InheritMiddleware = true
	}
}

// WithMiddleware adds additional middleware to the subrouter.
// These middleware run after inherited middleware but before route handlers.
func WithMiddleware(m ...Handler) MountOption {
	return func(cfg *MountConfig) {
		cfg.ExtraMiddleware = append(cfg.ExtraMiddleware, m...)
	}
}

// NamePrefix adds a prefix to all route names in the subrouter.
// Useful for metrics and logging scoping.
//
// Example:
//
//	r.Mount("/admin", sub, route.NamePrefix("admin."))
//	// Route named "users" becomes "admin.users"
func NamePrefix(prefix string) MountOption {
	return func(cfg *MountConfig) {
		cfg.NamePrefix = prefix
	}
}

// WithNotFound sets a custom 404 handler for the subrouter.
// This handler is only used when no route matches within the subrouter's prefix.
func WithNotFound(h Handler) MountOption {
	return func(cfg *MountConfig) {
		cfg.NotFoundHandler = h
	}
}

// BuildMountConfig applies mount options and returns the configuration.
func BuildMountConfig(opts ...MountOption) *MountConfig {
	cfg := &MountConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// MountRouteData contains route data prepared for mounting with a prefix.
// It holds all the information needed to register a route from a subrouter.
type MountRouteData struct {
	Method           string
	FullPath         string
	Handlers         []Handler
	Constraints      []Constraint
	TypedConstraints map[string]ParamConstraint
	Name             string
	Description      string
	Tags             []string
}

// PrepareMountRoute creates mount data from a route and prefix.
func PrepareMountRoute(prefix string, route *Route, middlewareChain []Handler, namePrefix string) *MountRouteData {
	// Build full path: prefix + route path
	var fullPath string
	if route.path == "/" {
		fullPath = prefix
	} else {
		fullPath = prefix + route.path
	}

	// Combine middleware chain with route handlers
	allHandlers := make([]Handler, 0, len(middlewareChain)+len(route.handlers))
	allHandlers = append(allHandlers, middlewareChain...)
	allHandlers = append(allHandlers, route.handlers...)

	data := &MountRouteData{
		Method:   route.method,
		FullPath: fullPath,
		Handlers: allHandlers,
	}

	// Copy constraints
	if len(route.constraints) > 0 {
		data.Constraints = make([]Constraint, len(route.constraints))
		copy(data.Constraints, route.constraints)
	}

	// Copy typed constraints
	if len(route.typedConstraints) > 0 {
		data.TypedConstraints = make(map[string]ParamConstraint, len(route.typedConstraints))
		maps.Copy(data.TypedConstraints, route.typedConstraints)
	}

	// Set name with prefix
	if route.name != "" {
		data.Name = namePrefix + route.name
	}

	// Copy metadata
	data.Description = route.description
	if len(route.tags) > 0 {
		data.Tags = make([]string, len(route.tags))
		copy(data.Tags, route.tags)
	}

	return data
}

// ExtractConstraintPattern extracts the pattern string from a compiled constraint.
func ExtractConstraintPattern(c Constraint) string {
	pattern := c.Pattern.String()
	if len(pattern) >= 2 && pattern[0] == '^' && pattern[len(pattern)-1] == '$' {
		return pattern[1 : len(pattern)-1]
	}

	return pattern
}
