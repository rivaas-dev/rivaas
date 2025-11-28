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
	"rivaas.dev/openapi/example"
	"rivaas.dev/router"
)

// RouteWrapper wraps a router.Route to provide both constraint
// and OpenAPI documentation methods in a unified, chainable API.
//
// Example:
//
//	app.GET("/users/:id", handler).
//	    Doc("Get user", "Retrieves a user by ID").
//	    Response(200, UserResponse{}).
//	    WhereInt("id")
type RouteWrapper struct {
	route   *router.Route
	openapi *openapi.RouteWrapper
}

// WhereInt adds a typed constraint that ensures the parameter is an integer.
//
// Example:
//
//	app.GET("/users/:id", handler).WhereInt("id")
func (rw *RouteWrapper) WhereInt(name string) *RouteWrapper {
	rw.route.WhereInt(name)
	return rw
}

// WhereFloat adds a typed constraint that ensures the parameter is a floating-point number.
//
// Example:
//
//	app.GET("/products/:price", handler).WhereFloat("price")
func (rw *RouteWrapper) WhereFloat(name string) *RouteWrapper {
	rw.route.WhereFloat(name)
	return rw
}

// WhereUUID adds a typed constraint that ensures the parameter is a valid UUID.
//
// Example:
//
//	app.GET("/users/:id", handler).WhereUUID("id")
func (rw *RouteWrapper) WhereUUID(name string) *RouteWrapper {
	rw.route.WhereUUID(name)
	return rw
}

// WhereRegex adds a typed constraint with a custom regex pattern.
//
// Example:
//
//	app.GET("/files/:name", handler).WhereRegex("name", `[a-zA-Z0-9._-]+`)
func (rw *RouteWrapper) WhereRegex(name, pattern string) *RouteWrapper {
	rw.route.WhereRegex(name, pattern)
	return rw
}

// WhereEnum adds a typed constraint that ensures the parameter matches one of the provided values.
//
// Example:
//
//	app.GET("/status/:state", handler).WhereEnum("state", "active", "inactive", "pending")
func (rw *RouteWrapper) WhereEnum(name string, values ...string) *RouteWrapper {
	rw.route.WhereEnum(name, values...)
	return rw
}

// WhereDate adds a typed constraint that ensures the parameter is an RFC3339 full-date.
//
// Example:
//
//	app.GET("/reports/:date", handler).WhereDate("date")
func (rw *RouteWrapper) WhereDate(name string) *RouteWrapper {
	rw.route.WhereDate(name)
	return rw
}

// WhereDateTime adds a typed constraint that ensures the parameter is an RFC3339 date-time.
//
// Example:
//
//	app.GET("/events/:timestamp", handler).WhereDateTime("timestamp")
func (rw *RouteWrapper) WhereDateTime(name string) *RouteWrapper {
	rw.route.WhereDateTime(name)
	return rw
}

// Where adds a regex constraint (legacy method, delegates to router.Route).
func (rw *RouteWrapper) Where(param, pattern string) *RouteWrapper {
	rw.route.Where(param, pattern)
	return rw
}

// Doc sets both summary and description for the route.
//
// Example:
//
//	app.GET("/users/:id", handler).
//	    Doc("Get user", "Retrieves a user by ID")
func (rw *RouteWrapper) Doc(summary, description string) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Doc(summary, description)
	}
	return rw
}

// Summary sets the summary for the route.
//
// Example:
//
//	app.GET("/users/:id", handler).Summary("Get user by ID")
func (rw *RouteWrapper) Summary(summary string) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Summary(summary)
	}
	return rw
}

// Description sets the description for the route.
//
// Example:
//
//	app.GET("/users/:id", handler).Description("Retrieves a user by their unique identifier")
func (rw *RouteWrapper) Description(description string) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Description(description)
	}
	return rw
}

// Tags adds tags to the route.
//
// Example:
//
//	app.GET("/users/:id", handler).Tags("users", "authentication")
func (rw *RouteWrapper) Tags(tags ...string) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Tags(tags...)
	}
	return rw
}

// Tag is an alias for Tags for convenience.
//
// Example:
//
//	app.GET("/users/:id", handler).Tag("users")
func (rw *RouteWrapper) Tag(tags ...string) *RouteWrapper {
	return rw.Tags(tags...)
}

// Response sets the response schema and examples for a status code.
//
// The schema is generated from resp's type. Examples are handled as follows:
//   - No examples provided: resp itself is used as the example
//   - Examples provided: they become named examples in OpenAPI
//
// Example (simple):
//
//	app.GET("/users/:id", handler).
//	    Response(200, UserResponse{ID: 123, Name: "John"})
//
// Example (named examples):
//
//	app.GET("/users/:id", handler).
//	    Response(200, UserResponse{},
//	        example.New("success", UserResponse{ID: 123}, example.WithSummary("Found")),
//	        example.New("admin", UserResponse{ID: 1, Role: "admin"}),
//	    )
func (rw *RouteWrapper) Response(status int, resp any, examples ...example.Example) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Response(status, resp, examples...)
	}
	return rw
}

// Request sets the request type and optionally provides examples.
//
// The schema is generated from req's type. Examples are handled as follows:
//   - No examples provided: req itself is used as the example
//   - Examples provided: they become named examples in OpenAPI
//
// Example (simple):
//
//	app.POST("/users", handler).
//	    Request(CreateUserRequest{Name: "John", Email: "john@example.com"})
//
// Example (named examples):
//
//	app.POST("/users", handler).
//	    Request(CreateUserRequest{},
//	        example.New("minimal", CreateUserRequest{Name: "John"}),
//	        example.New("complete", CreateUserRequest{Name: "John", Email: "john@example.com", Age: 30}),
//	    )
func (rw *RouteWrapper) Request(req any, examples ...example.Example) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Request(req, examples...)
	}
	return rw
}

// Security adds security requirements to the route.
// The first argument is the scheme name, followed by optional scopes.
//
// Example:
//
//	app.GET("/users/:id", handler).
//	    Security("bearerAuth", "read:users")
func (rw *RouteWrapper) Security(scheme string, scopes ...string) *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Security(scheme, scopes...)
	}
	return rw
}

// Deprecated marks the route as deprecated.
//
// Example:
//
//	app.GET("/old-endpoint", handler).Deprecated()
func (rw *RouteWrapper) Deprecated() *RouteWrapper {
	if rw.openapi != nil {
		rw.openapi.Deprecated()
	}
	return rw
}

// Route returns the underlying router.Route.
//
// Example:
//
//	route := app.GET("/users/:id", handler).Route()
//	route.SetName("get-user")
func (rw *RouteWrapper) Route() *router.Route {
	return rw.route
}

// OpenAPI returns the underlying openapi.RouteWrapper.
//
// Example:
//
//	oapi := app.GET("/users/:id", handler).OpenAPI()
//	if oapi != nil {
//	    oapi.Summary("Get user")
//	}
func (rw *RouteWrapper) OpenAPI() *openapi.RouteWrapper {
	return rw.openapi
}
