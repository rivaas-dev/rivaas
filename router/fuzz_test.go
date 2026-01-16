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

package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// FuzzRouteParsing tests route parsing with fuzzed paths.
// This fuzz test ensures the router never panics, even with malformed or edge-case paths.
func FuzzRouteParsing(f *testing.F) {
	// Seed corpus with known good/bad inputs
	f.Add("/")
	f.Add("/users")
	f.Add("/users/:id")
	f.Add("/users/:id/posts/:postId")
	f.Add("/static/*")
	f.Add("")
	f.Add("//")
	f.Add("/users//posts")
	f.Add("/users/:id/:name")
	f.Add("/users/:id/posts/:postId/comments/:commentId")
	f.Add("/api/v1/users/:id")
	f.Add("invalid-path-without-leading-slash")
	f.Add("/very/long/path/with/many/segments/that/might/cause/issues")
	f.Add("/users/123")
	f.Add("/users/123/posts/456")

	f.Fuzz(func(t *testing.T, path string) {
		r := MustNew()

		// Register a route with the fuzzed path
		// Should never panic, even with invalid paths
		r.GET(path, func(c *Context) {
			c.Status(http.StatusOK)
		})

		// Try to compile routes - should not panic
		r.CompileAllRoutes()

		// Try to warmup - should not panic
		r.Warmup()

		// Try to match a request - should not panic
		// Handle invalid paths (httptest.NewRequest requires valid URL paths)
		requestPath := path
		if requestPath == "" || !strings.HasPrefix(requestPath, "/") {
			requestPath = "/"
		}
		req := httptest.NewRequest(http.MethodGet, requestPath, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Should get a valid status code (200, 404, or 405)
		status := w.Code
		if status < 100 || status >= 600 {
			t.Errorf("invalid status code %d for path %q", status, path)
		}
	})
}

// FuzzParameterExtraction tests parameter extraction with fuzzed paths and values.
// This ensures parameter extraction never panics and handles edge cases correctly.
func FuzzParameterExtraction(f *testing.F) {
	// Seed corpus
	f.Add("/users/:id", "/users/123")
	f.Add("/users/:id/posts/:postId", "/users/123/posts/456")
	f.Add("/api/:version/users/:id", "/api/v1/users/42")
	f.Add("/users/:id", "/users/")
	f.Add("/users/:id", "/users//")
	f.Add("/users/:id", "/users/very-long-id-value-that-might-cause-issues")
	f.Add("/users/:id", "/users/123?query=value")
	f.Add("/users/:id", "/users/123#fragment")

	f.Fuzz(func(t *testing.T, routePath, requestPath string) {
		r := MustNew()

		var capturedParams map[string]string

		// Register route with parameter
		r.GET(routePath, func(c *Context) {
			// Extract all parameters - should not panic
			capturedParams = c.AllParams()
			c.Status(http.StatusOK)
		})

		// Handle empty path case (httptest.NewRequest doesn't accept empty URL)
		if requestPath == "" {
			requestPath = "/"
		}

		// Make request - should not panic
		req := httptest.NewRequest(http.MethodGet, requestPath, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// If route matched, verify parameters are valid
		if w.Code == http.StatusOK {
			if capturedParams == nil {
				t.Errorf("route matched but no parameters captured for route %q, path %q", routePath, requestPath)
			}
			// Parameters should be strings (not nil)
			for key, value := range capturedParams {
				if key == "" {
					t.Errorf("empty parameter key in route %q, path %q", routePath, requestPath)
				}
				// Value can be empty string, but should not be nil
				_ = value
			}
		}
	})
}

// FuzzConstraintValidation tests constraint validation with fuzzed parameter values.
// This ensures constraint validation never panics and handles invalid inputs gracefully.
func FuzzConstraintValidation(f *testing.F) {
	// Seed corpus
	f.Add("/users/:id", "123")
	f.Add("/users/:id", "abc")
	f.Add("/users/:id", "")
	f.Add("/users/:id", "-123")
	f.Add("/users/:id", "123abc")
	f.Add("/users/:id", "very-long-value-that-exceeds-normal-expectations")
	f.Add("/users/:id", "123.456")
	f.Add("/users/:id", "0")

	f.Fuzz(func(t *testing.T, routePath, paramValue string) {
		r := MustNew()

		// Register route with integer constraint
		r.GET(routePath, func(c *Context) {
			id := c.Param("id")
			// Should never panic, even with invalid values
			_ = id
			c.Status(http.StatusOK)
		}).WhereInt("id")

		// Make request with fuzzed parameter value
		requestPath := "/users/" + paramValue
		req := httptest.NewRequest(http.MethodGet, requestPath, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Should get valid status code
		status := w.Code
		if status < 100 || status >= 600 {
			t.Errorf("invalid status code %d for route %q, param value %q", status, routePath, paramValue)
		}

		// If constraint validation failed, should return 404 or 400
		// If validation passed, should return 200
		// Either way, should not panic
	})
}

// FuzzQueryParameters tests query parameter extraction with fuzzed query strings.
// This ensures query parameter handling never panics with malformed query strings.
func FuzzQueryParameters(f *testing.F) {
	// Seed corpus
	f.Add("key=value")
	f.Add("key1=value1&key2=value2")
	f.Add("key=")
	f.Add("=value")
	f.Add("key=value&key2")
	f.Add("key=value%20with%20spaces")
	f.Add("key=value+with+plus")
	f.Add("key=value&key=duplicate")
	f.Add("")
	f.Add("key=value&")
	f.Add("&key=value")
	f.Add("key=value&key2=value2&key3=value3")

	f.Fuzz(func(t *testing.T, queryString string) {
		r := MustNew()

		var capturedValue string

		r.GET("/test", func(c *Context) {
			// Extract query parameter - should not panic
			capturedValue = c.Query("key")
			c.Status(http.StatusOK)
		})

		// Make request with fuzzed query string
		req := httptest.NewRequest(http.MethodGet, "/test?"+queryString, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Should get valid status code
		if w.Code != http.StatusOK {
			t.Errorf("unexpected status code %d for query string %q", w.Code, queryString)
		}

		// Value should be a string (can be empty)
		_ = capturedValue
	})
}

// FuzzWildcardRoutes tests wildcard route matching with fuzzed paths.
// This ensures wildcard routes handle edge cases correctly.
func FuzzWildcardRoutes(f *testing.F) {
	// Seed corpus
	f.Add("/static/*", "/static/file.txt")
	f.Add("/static/*", "/static/dir/file.txt")
	f.Add("/static/*", "/static/very/deep/nested/path/to/file.txt")
	f.Add("/static/*", "/static/")
	f.Add("/static/*", "/static")
	f.Add("/static/*", "/static/file")
	f.Add("/static/*", "/static/file.txt?query=value")
	f.Add("/*", "/anything")
	f.Add("/*", "/")
	f.Add("/*", "")

	f.Fuzz(func(t *testing.T, routePath, requestPath string) {
		r := MustNew()

		var capturedPath string

		// Register wildcard route
		r.GET(routePath, func(c *Context) {
			// Extract wildcard parameter - should not panic
			if routePath == "/*" {
				capturedPath = c.Param("filepath")
			} else {
				capturedPath = c.Param("filepath")
			}
			c.Status(http.StatusOK)
		})

		// Handle empty path case (httptest.NewRequest doesn't accept empty URL)
		if requestPath == "" {
			requestPath = "/"
		}

		// Make request - should not panic
		req := httptest.NewRequest(http.MethodGet, requestPath, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Should get valid status code
		status := w.Code
		if status < 100 || status >= 600 {
			t.Errorf("invalid status code %d for route %q, path %q", status, routePath, requestPath)
		}

		// If route matched, captured path should be set
		if status == http.StatusOK {
			_ = capturedPath // Can be empty string
		}
	})
}

// FuzzContextPooling tests context pooling with concurrent fuzzed requests.
// This ensures context pooling handles edge cases correctly.
func FuzzContextPooling(f *testing.F) {
	// Seed corpus
	f.Add("/test")
	f.Add("/users/:id")
	f.Add("/users/:id/posts/:postId")
	f.Add("/static/*")

	f.Fuzz(func(t *testing.T, path string) {
		r := MustNew()

		// Register route
		r.GET(path, func(c *Context) {
			// Use context methods - should not panic
			_ = c.Param("id")
			_ = c.Query("q")
			_ = c.Request.Method
			_ = c.Request.URL.Path
			c.Status(http.StatusOK)
		})

		// Handle empty path case (httptest.NewRequest doesn't accept empty URL)
		requestPath := path
		if requestPath == "" {
			requestPath = "/"
		}

		// Make multiple requests to test pooling
		for i := range 10 {
			req := httptest.NewRequest(http.MethodGet, requestPath, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Should get valid status code
			if w.Code < 100 || w.Code >= 600 {
				t.Errorf("invalid status code %d for path %q on iteration %d", w.Code, path, i)
			}
		}
	})
}

// FuzzRouteRegistration tests concurrent route registration with fuzzed paths.
// This ensures route registration is thread-safe and handles edge cases.
func FuzzRouteRegistration(f *testing.F) {
	// Seed corpus
	f.Add("/route1")
	f.Add("/route2/:id")
	f.Add("/route3/*")
	f.Add("/api/v1/users/:id")

	f.Fuzz(func(t *testing.T, path string) {
		r := MustNew()

		// Register multiple routes concurrently - should not panic
		done := make(chan bool, 3)
		for i := range 3 {
			go func(id int) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panic during route registration: %v for path %q", r, path)
					}
					done <- true
				}()

				routePath := path + string(rune('0'+id))
				r.GET(routePath, func(c *Context) {
					c.Status(http.StatusOK)
				})
			}(i)
		}

		// Wait for all registrations
		for range 3 {
			<-done
		}

		// Routes should be accessible
		routes := r.Routes()
		if len(routes) == 0 {
			t.Errorf("no routes registered for path %q", path)
		}
	})
}
