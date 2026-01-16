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

//go:build !integration

package compiler_test

import (
	"fmt"
	"regexp"

	"rivaas.dev/router/compiler"
)

// ExampleNewRouteCompiler demonstrates creating a new route compiler.
func ExampleNewRouteCompiler() {
	// Create a route compiler with bloom filter size 1000 and 3 hash functions
	rc := compiler.NewRouteCompiler(1000, 3)

	// Add routes to the compiler
	route := compiler.CompileRoute("GET", "/users", nil, nil)
	rc.AddRoute(route)

	fmt.Println("Route compiler created successfully")
	// Output: Route compiler created successfully
}

// ExampleCompileRoute demonstrates compiling a simple static route.
func ExampleCompileRoute() {
	// Compile a static route (no parameters)
	route := compiler.CompileRoute("GET", "/api/users", nil, nil)

	fmt.Println("Pattern:", route.Pattern())
	fmt.Println("Method:", route.Method())
	// Output:
	// Pattern: /api/users
	// Method: GET
}

// ExampleCompileRoute_withParameters demonstrates compiling a route with parameters.
func ExampleCompileRoute_withParameters() {
	// Compile a route with URL parameters
	route := compiler.CompileRoute("GET", "/users/:id/posts/:pid", nil, nil)

	fmt.Println("Pattern:", route.Pattern())
	fmt.Println("Method:", route.Method())
	// Output:
	// Pattern: /users/:id/posts/:pid
	// Method: GET
}

// ExampleCompileRoute_withConstraints demonstrates compiling a route with parameter constraints.
func ExampleCompileRoute_withConstraints() {
	// Define constraints for parameters
	constraints := []compiler.RouteConstraint{
		{Param: "id", Pattern: regexp.MustCompile(`^\d+$`)},  // id must be numeric
		{Param: "pid", Pattern: regexp.MustCompile(`^\d+$`)}, // pid must be numeric
	}

	// Compile route with constraints
	route := compiler.CompileRoute("GET", "/users/:id/posts/:pid", nil, constraints)

	fmt.Println("Pattern:", route.Pattern())
	fmt.Println("Method:", route.Method())
	// Output:
	// Pattern: /users/:id/posts/:pid
	// Method: GET
}

// ExampleRouteCompiler_AddRoute demonstrates adding routes to a compiler.
func ExampleRouteCompiler_AddRoute() {
	rc := compiler.NewRouteCompiler(1000, 3)

	// Add a static route
	staticRoute := compiler.CompileRoute("GET", "/health", nil, nil)
	rc.AddRoute(staticRoute)

	// Add a dynamic route
	dynamicRoute := compiler.CompileRoute("GET", "/users/:id", nil, nil)
	rc.AddRoute(dynamicRoute)

	fmt.Println("Routes added successfully")
	// Output: Routes added successfully
}

// ExampleRouteCompiler_LookupStatic demonstrates looking up static routes.
func ExampleRouteCompiler_LookupStatic() {
	rc := compiler.NewRouteCompiler(1000, 3)

	// Add static routes
	route := compiler.CompileRoute("GET", "/health", nil, nil)
	rc.AddRoute(route)

	// Lookup the route
	found := rc.LookupStatic("GET", "/health")
	if found != nil {
		fmt.Println("Found route:", found.Pattern())
	}

	// Lookup non-existent route
	notFound := rc.LookupStatic("GET", "/nonexistent")
	if notFound == nil {
		fmt.Println("Route not found")
	}
	// Output:
	// Found route: /health
	// Route not found
}

// ExampleNewBloomFilter demonstrates creating and using a bloom filter.
func ExampleNewBloomFilter() {
	// Create a bloom filter with size 1000 and 3 hash functions
	bf := compiler.NewBloomFilter(1000, 3)

	// Add items to the filter
	bf.Add([]byte("GET/users"))
	bf.Add([]byte("GET/posts"))

	// Test membership
	if bf.Test([]byte("GET/users")) {
		fmt.Println("GET/users: possibly in set")
	}

	if !bf.Test([]byte("GET/comments")) {
		fmt.Println("GET/comments: definitely not in set")
	}
	// Output:
	// GET/users: possibly in set
	// GET/comments: definitely not in set
}
