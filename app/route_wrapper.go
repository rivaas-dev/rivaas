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

package app

import (
	"rivaas.dev/openapi"
	"rivaas.dev/router"
)

// RouteWrapper wraps a router.Route to provide both constraint
// and OpenAPI documentation methods in a unified, chainable API.
type RouteWrapper struct {
	route   *router.Route
	openapi *openapi.RouteWrapper
}

// ---- Constraint methods (delegate to router.Route) ----

// WhereInt adds a typed constraint that ensures the parameter is an integer.
func (rw *RouteWrapper) WhereInt(name string) *RouteWrapper {
	rw.route.WhereInt(name)
	return rw
}

// WhereFloat adds a typed constraint that ensures the parameter is a floating-point number.
func (rw *RouteWrapper) WhereFloat(name string) *RouteWrapper {
	rw.route.WhereFloat(name)
	return rw
}

// WhereUUID adds a typed constraint that ensures the parameter is a valid UUID.
func (rw *RouteWrapper) WhereUUID(name string) *RouteWrapper {
	rw.route.WhereUUID(name)
	return rw
}

// WhereRegex adds a typed constraint with a custom regex pattern.
func (rw *RouteWrapper) WhereRegex(name, pattern string) *RouteWrapper {
	rw.route.WhereRegex(name, pattern)
	return rw
}

// WhereEnum adds a typed constraint that ensures the parameter matches one of the provided values.
func (rw *RouteWrapper) WhereEnum(name string, values ...string) *RouteWrapper {
	rw.route.WhereEnum(name, values...)
	return rw
}

// WhereDate adds a typed constraint that ensures the parameter is an RFC3339 full-date.
func (rw *RouteWrapper) WhereDate(name string) *RouteWrapper {
	rw.route.WhereDate(name)
	return rw
}

// WhereDateTime adds a typed constraint that ensures the parameter is an RFC3339 date-time.
func (rw *RouteWrapper) WhereDateTime(name string) *RouteWrapper {
	rw.route.WhereDateTime(name)
	return rw
}

// Where adds a regex constraint (legacy method, delegates to router.Route).
func (rw *RouteWrapper) Where(param, pattern string) *RouteWrapper {
	rw.route.Where(param, pattern)
	return rw
}

// WhereNumber adds a constraint that ensures the parameter is numeric (legacy method).
func (rw *RouteWrapper) WhereNumber(param string) *RouteWrapper {
	rw.route.WhereNumber(param)
	return rw
}

// WhereAlpha adds a constraint that ensures the parameter contains only letters (legacy method).
func (rw *RouteWrapper) WhereAlpha(param string) *RouteWrapper {
	rw.route.WhereAlpha(param)
	return rw
}

// WhereAlphaNumeric adds a constraint that ensures the parameter contains only letters and numbers (legacy method).
func (rw *RouteWrapper) WhereAlphaNumeric(param string) *RouteWrapper {
	rw.route.WhereAlphaNumeric(param)
	return rw
}

// ---- OpenAPI methods (delegate to openapi.RouteWrapper) ----

// Doc sets both summary and description for the route.
func (rw *RouteWrapper) Doc(summary, description string) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Doc(summary, description)
	}
	return rw
}

// Summary sets the summary for the route.
func (rw *RouteWrapper) Summary(summary string) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Summary(summary)
	}
	return rw
}

// Description sets the description for the route.
func (rw *RouteWrapper) Description(description string) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Description(description)
	}
	return rw
}

// Tags adds tags to the route.
func (rw *RouteWrapper) Tags(tags ...string) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Tags(tags...)
	}
	return rw
}

// Tag is an alias for Tags for convenience.
func (rw *RouteWrapper) Tag(tags ...string) *RouteWrapper {
	return rw.Tags(tags...)
}

// Response adds a response example for the given status code.
func (rw *RouteWrapper) Response(status int, example any) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Response(status, example)
	}
	return rw
}

// Request sets the request body example.
func (rw *RouteWrapper) Request(example any) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Request(example)
	}
	return rw
}

// Security adds security requirements to the route.
// The first argument is the scheme name, followed by optional scopes.
func (rw *RouteWrapper) Security(scheme string, scopes ...string) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Security(scheme, scopes...)
	}
	return rw
}

// Deprecated marks the route as deprecated.
func (rw *RouteWrapper) Deprecated() *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Deprecated()
	}
	return rw
}

// ---- Access underlying objects if needed ----

// Route returns the underlying router.Route.
func (rw *RouteWrapper) Route() *router.Route {
	return rw.route
}

// OpenAPI returns the underlying openapi.RouteWrapper.
func (rw *RouteWrapper) OpenAPI() *openapi.RouteWrapper {
	return rw.openapi
}
