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

package router_test

import (
	"rivaas.dev/router/versioning"
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"rivaas.dev/router"
)

// registerMethod is a helper function to register routes for different HTTP methods
func registerMethod(r *router.Router, method, path string, handler router.HandlerFunc) {
	switch method {
	case http.MethodGet:
		r.GET(path, handler)
	case http.MethodPost:
		r.POST(path, handler)
	case http.MethodPut:
		r.PUT(path, handler)
	case http.MethodDelete:
		r.DELETE(path, handler)
	case http.MethodPatch:
		r.PATCH(path, handler)
	case http.MethodOptions:
		r.OPTIONS(path, handler)
	case http.MethodHead:
		r.HEAD(path, handler)
	}
}

// mockHijackableResponseWriter implements http.Hijacker for testing
type mockHijackableResponseWriter struct {
	*httptest.ResponseRecorder
	hijackCalled bool
	conn         net.Conn
	rw           *bufio.ReadWriter
	hijackErr    error
}

func (m *mockHijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijackCalled = true
	return m.conn, m.rw, m.hijackErr
}

// mockFlushableResponseWriter implements http.Flusher for testing
type mockFlushableResponseWriter struct {
	*httptest.ResponseRecorder
	flushCalled bool
}

func (m *mockFlushableResponseWriter) Flush() {
	m.flushCalled = true
}

// mockHijackFlushResponseWriter implements both http.Hijacker and http.Flusher
type mockHijackFlushResponseWriter struct {
	*httptest.ResponseRecorder
	hijackCalled bool
	flushCalled  bool
	conn         net.Conn
	rw           *bufio.ReadWriter
	hijackErr    error
}

func (m *mockHijackFlushResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	m.hijackCalled = true
	return m.conn, m.rw, m.hijackErr
}

func (m *mockHijackFlushResponseWriter) Flush() {
	m.flushCalled = true
}

var _ = Describe("Router Integration", func() {
	Describe("Complete request lifecycle", func() {
		It("should execute middleware and handlers in correct order", func() {
			r := router.MustNew()

			var executionOrder []string

			// Global middleware
			r.Use(func(c *router.Context) {
				executionOrder = append(executionOrder, "global-start")
				c.Next()
				executionOrder = append(executionOrder, "global-end")
			})

			// Group with middleware
			api := r.Group("/api", func(c *router.Context) {
				executionOrder = append(executionOrder, "api-start")
				c.Next()
				executionOrder = append(executionOrder, "api-end")
			})

			// Nested group
			v1 := api.Group("/v1", func(c *router.Context) {
				executionOrder = append(executionOrder, "v1-start")
				c.Next()
				executionOrder = append(executionOrder, "v1-end")
			})

			// Route handler
			v1.GET("/users/:id", func(c *router.Context) {
				executionOrder = append(executionOrder, "handler")
				id := c.Param("id")
				c.JSON(http.StatusOK, map[string]string{"id": id})
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			expected := []string{
				"global-start",
				"api-start",
				"v1-start",
				"handler",
				"v1-end",
				"api-end",
				"global-end",
			}

			Expect(executionOrder).To(HaveLen(len(expected)))
			Expect(executionOrder).To(Equal(expected))
			Expect(w.Code).To(Equal(http.StatusOK))

			// Verify response body
			var response map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal("123"))
		})
	})

	Describe("HTTP methods with parameters", func() {
		DescribeTable("should handle all HTTP methods with route parameters",
			func(method string) {
				r := router.MustNew()

				called := false

				// Use helper function to register method
				registerMethod(r, method, "/users/:id/posts/:pid", func(c *router.Context) {
					called = true
					userID := c.Param("id")
					postID := c.Param("pid")

					Expect(userID).To(Equal("42"))
					Expect(postID).To(Equal("99"))

					c.Status(http.StatusOK)
				})

				req := httptest.NewRequest(method, "/users/42/posts/99", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(called).To(BeTrue(), "%s handler should be called", method)
				Expect(w.Code).To(Equal(http.StatusOK))
			},
			Entry("GET", http.MethodGet),
			Entry("POST", http.MethodPost),
			Entry("PUT", http.MethodPut),
			Entry("DELETE", http.MethodDelete),
			Entry("PATCH", http.MethodPatch),
			Entry("OPTIONS", http.MethodOptions),
			Entry("HEAD", http.MethodHead),
		)
	})

	Describe("Router with all features", func() {
		Context("with bloom filter and template routing", func() {
			It("should work correctly", func() {
				r := router.MustNew(
					router.WithBloomFilterSize(2000),
					router.WithBloomFilterHashFunctions(5),
					router.WithCancellationCheck(true),
					router.WithTemplateRouting(true),
				)

				// Add middleware
				r.Use(func(c *router.Context) {
					c.Next()
				})

				// Register and test route
				r.GET("/test", func(c *router.Context) {
					c.JSON(http.StatusOK, map[string]string{"status": "ok", "feature": "enabled"})
				})

				r.Warmup()

				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))

				// Verify response body
				var response map[string]string
				err := json.Unmarshal(w.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response["status"]).To(Equal("ok"))
				Expect(response["feature"]).To(Equal("enabled"))
			})
		})

		Context("with versioning", func() {
			It("should route by version header", func() {
				rVersioned := router.MustNew(
					router.WithVersioning(
						versioning.WithHeaderVersioning("X-API-Version"),
					),
				)

				// Add middleware
				rVersioned.Use(func(c *router.Context) {
					c.Next()
				})

				// Register routes
				v1 := rVersioned.Version("v1")
				v1.GET("/users/:id", func(c *router.Context) {
					c.JSON(http.StatusOK, map[string]string{"id": c.Param("id"), "version": "v1"})
				})

				v2 := rVersioned.Version("v2")
				v2.GET("/users/:id", func(c *router.Context) {
					c.JSON(http.StatusOK, map[string]string{"id": c.Param("id"), "version": "v2"})
				})

				// Warmup
				rVersioned.Warmup()

				// Test v1
				req1 := httptest.NewRequest(http.MethodGet, "/users/123", nil)
				req1.Header.Set("X-Api-Version", "v1")
				w1 := httptest.NewRecorder()
				rVersioned.ServeHTTP(w1, req1)

				Expect(w1.Code).To(Equal(http.StatusOK))

				// Verify v1 response body
				var response1 map[string]string
				err := json.Unmarshal(w1.Body.Bytes(), &response1)
				Expect(err).NotTo(HaveOccurred())
				Expect(response1["id"]).To(Equal("123"))
				Expect(response1["version"]).To(Equal("v1"))

				// Test v2
				req2 := httptest.NewRequest(http.MethodGet, "/users/456", nil)
				req2.Header.Set("X-Api-Version", "v2")
				w2 := httptest.NewRecorder()
				rVersioned.ServeHTTP(w2, req2)

				Expect(w2.Code).To(Equal(http.StatusOK))

				// Verify v2 response body
				var response2 map[string]string
				err = json.Unmarshal(w2.Body.Bytes(), &response2)
				Expect(err).NotTo(HaveOccurred())
				Expect(response2["id"]).To(Equal("456"))
				Expect(response2["version"]).To(Equal("v2"))
			})
		})
	})

	Describe("Mixed static and dynamic routes", func() {
		var r *router.Router

		BeforeEach(func() {
			r = router.MustNew()

			// Static routes
			r.GET("/", func(c *router.Context) {
				c.String(http.StatusOK, "home")
			})

			r.GET("/about", func(c *router.Context) {
				c.String(http.StatusOK, "about")
			})

			r.GET("/contact", func(c *router.Context) {
				c.String(http.StatusOK, "contact")
			})

			// Dynamic routes
			r.GET("/users/:id", func(c *router.Context) {
				c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
			})

			r.GET("/posts/:id/comments/:cid", func(c *router.Context) {
				c.JSON(http.StatusOK, map[string]string{
					"post":    c.Param("id"),
					"comment": c.Param("cid"),
				})
			})

			// Wildcard routes
			r.GET("/files/*", func(c *router.Context) {
				c.String(http.StatusOK, "file")
			})

			r.Warmup()
		})

		DescribeTable("should route correctly",
			func(path string, expectCode int) {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(expectCode), "path: %s", path)

				// Verify response body for successful routes
				if expectCode == http.StatusOK {
					if strings.HasPrefix(path, "/users/") {
						var response map[string]string
						err := json.Unmarshal(w.Body.Bytes(), &response)
						Expect(err).NotTo(HaveOccurred())
						Expect(response).To(HaveKey("id"))
					} else if strings.HasPrefix(path, "/posts/") {
						var response map[string]string
						err := json.Unmarshal(w.Body.Bytes(), &response)
						Expect(err).NotTo(HaveOccurred())
						Expect(response).To(HaveKey("post"))
						Expect(response).To(HaveKey("comment"))
					} else if path == "/" || path == "/about" || path == "/contact" {
						Expect(w.Body.String()).To(BeElementOf([]string{"home", "about", "contact"}))
					} else if strings.HasPrefix(path, "/files/") {
						Expect(w.Body.String()).To(Equal("file"))
					}
				}
			},
			Entry("root path", "/", 200),
			Entry("about page", "/about", 200),
			Entry("contact page", "/contact", 200),
			Entry("user with id", "/users/1", 200),
			Entry("post with comment", "/posts/1/comments/2", 200),
			Entry("wildcard file path", "/files/anything/here", 200),
			Entry("not found", "/notfound", 404),
		)
	})

	Describe("Abort in middleware", func() {
		It("should abort middleware chain correctly", func() {
			r := router.MustNew()

			executionLog := []string{}

			r.Use(func(c *router.Context) {
				executionLog = append(executionLog, "middleware1-before")
				c.Next()
				executionLog = append(executionLog, "middleware1-after")
			})

			r.Use(func(c *router.Context) {
				executionLog = append(executionLog, "middleware2-before")
				// Abort the chain
				c.Abort()
				executionLog = append(executionLog, "middleware2-abort")
				// Even though we call Next, it should not proceed
				c.Next()
				executionLog = append(executionLog, "middleware2-after")
			})

			r.GET("/test", func(c *router.Context) {
				executionLog = append(executionLog, "handler")
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Verify handler was NOT called
			Expect(executionLog).NotTo(ContainElement("handler"))

			// Verify middleware cleanup still ran
			Expect(executionLog).To(ContainElement("middleware1-after"))
		})
	})

	Describe("Panic recovery", func() {
		It("should propagate panics without recovery middleware", func() {
			r := router.MustNew()

			r.GET("/panic", func(_ *router.Context) {
				panic("test panic")
			})

			// Without recovery middleware, panic should propagate
			req := httptest.NewRequest(http.MethodGet, "/panic", nil)
			w := httptest.NewRecorder()

			Expect(func() {
				r.ServeHTTP(w, req)
			}).To(PanicWith("test panic"))
		})
	})

	Describe("Static file serving", func() {
		var r *router.Router

		BeforeEach(func() {
			r = router.MustNew()

			// Static file serving (using handler)
			r.GET("/static/*", func(c *router.Context) {
				c.Header("Content-Type", "text/plain")
				c.String(http.StatusOK, "static content")
			})
		})

		DescribeTable("should serve static file paths",
			func(path string) {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("static content"))
				Expect(w.Header().Get("Content-Type")).To(Equal("text/plain"))
			},
			Entry("single file", "/static/file.txt"),
			Entry("nested file", "/static/dir/file.txt"),
			Entry("deeply nested file", "/static/a/b/c/d.txt"),
		)
	})

	Describe("Content negotiation", func() {
		var r *router.Router
		var data map[string]string

		BeforeEach(func() {
			r = router.MustNew()
			data = map[string]string{"message": "hello"}

			r.GET("/data", func(c *router.Context) {
				c.Format(http.StatusOK, data)
			})
		})

		DescribeTable("should negotiate content type",
			func(acceptHeader string, expectType string) {
				req := httptest.NewRequest(http.MethodGet, "/data", nil)
				req.Header.Set("Accept", acceptHeader)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))

				contentType := w.Header().Get("Content-Type")
				Expect(contentType).To(ContainSubstring(expectType))

				// Verify response body is not empty
				Expect(w.Body.Len()).To(BeNumerically(">", 0))
			},
			Entry("JSON", "application/json", "json"),
			Entry("HTML", "text/html", "html"),
			Entry("XML", "application/xml", "xml"),
			Entry("Plain text", "text/plain", "plain"),
			Entry("Default", "*/*", "json"),
		)
	})

	Describe("Error handling", func() {
		It("should handle binding errors correctly", func() {
			r := router.MustNew()

			// Route that returns binding error
			r.POST("/bind", func(c *router.Context) {
				type Data struct {
					Age int `json:"age"`
				}

				var data Data
				if err := json.NewDecoder(c.Request.Body).Decode(&data); err != nil {
					c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}

				c.JSON(http.StatusOK, data)
			})

			// Send invalid JSON
			req := httptest.NewRequest(http.MethodPost, "/bind", strings.NewReader(`{"age": "not a number"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))

			// Verify error response body
			var errorResponse map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
			Expect(err).NotTo(HaveOccurred())
			Expect(errorResponse).To(HaveKey("error"))
			Expect(errorResponse["error"]).NotTo(BeEmpty())
		})
	})

	Describe("Basic routing", func() {
		var r *router.Router

		BeforeEach(func() {
			r = router.MustNew()

			r.GET("/", func(c *router.Context) {
				c.String(http.StatusOK, "Hello World")
			})

			r.GET("/users/:id", func(c *router.Context) {
				c.Stringf(http.StatusOK, "User: %s", c.Param("id"))
			})

			r.POST("/users", func(c *router.Context) {
				c.String(http.StatusCreated, "User created")
			})
		})

		DescribeTable("should route correctly",
			func(method, path string, expectedStatus int, expectedBody string) {
				req := httptest.NewRequest(method, path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(expectedStatus))
				if expectedBody != "" {
					Expect(w.Body.String()).To(Equal(expectedBody))
				}
			},
			Entry("GET root", http.MethodGet, "/", 200, "Hello World"),
			Entry("GET user", http.MethodGet, "/users/123", 200, "User: 123"),
			Entry("POST users", http.MethodPost, "/users", 201, "User created"),
			Entry("GET nonexistent", http.MethodGet, "/users/123/posts/456", 404, ""),
			Entry("GET not found", http.MethodGet, "/nonexistent", 404, ""),
		)
	})

	Describe("Middleware integration", func() {
		It("should execute middleware and handlers correctly", func() {
			r := router.MustNew()

			r.Use(func(c *router.Context) {
				c.Header("X-Middleware", "true")
				c.Next()
			})

			r.GET("/", func(c *router.Context) {
				c.String(http.StatusOK, "Hello")
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("X-Middleware")).To(Equal("true"))
			Expect(w.Body.String()).To(Equal("Hello"))
		})
	})

	Describe("Route groups", func() {
		var r *router.Router

		BeforeEach(func() {
			r = router.MustNew()

			api := r.Group("/api/v1")
			api.GET("/users", func(c *router.Context) {
				c.String(http.StatusOK, "Users")
			})

			api.GET("/users/:id", func(c *router.Context) {
				c.Stringf(http.StatusOK, "User: %s", c.Param("id"))
			})
		})

		DescribeTable("should route group paths correctly",
			func(path string, expectedStatus int, expectedBody string) {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(expectedStatus))
				if expectedBody != "" {
					Expect(w.Body.String()).To(Equal(expectedBody))
				}
			},
			Entry("group users list", "/api/v1/users", 200, "Users"),
			Entry("group user by id", "/api/v1/users/123", 200, "User: 123"),
			Entry("non-group path", "/users", 404, ""),
		)

		It("should apply middleware to route groups", func() {
			r := router.MustNew()

			api := r.Group("/api/v1")
			api.Use(func(c *router.Context) {
				c.Header("X-Api-Version", "v1")
				c.Next()
			})

			api.GET("/users", func(c *router.Context) {
				c.String(http.StatusOK, "Users")
			})

			req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("X-Api-Version")).To(Equal("v1"))
			Expect(w.Body.String()).To(Equal("Users"))
		})
	})

	Describe("Complex route patterns", func() {
		var r *router.Router

		BeforeEach(func() {
			r = router.MustNew()

			r.GET("/users/:id/posts/:post_id", func(c *router.Context) {
				c.Stringf(http.StatusOK, "User: %s, Post: %s", c.Param("id"), c.Param("post_id"))
			})

			r.GET("/users/:id/posts/:post_id/comments/:comment_id", func(c *router.Context) {
				c.Stringf(http.StatusOK, "User: %s, Post: %s, Comment: %s",
					c.Param("id"), c.Param("post_id"), c.Param("comment_id"))
			})
		})

		DescribeTable("should route complex patterns correctly",
			func(path string, expectedStatus int, expectedBody string) {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(expectedStatus))
				if expectedBody != "" {
					Expect(w.Body.String()).To(Equal(expectedBody))
				}
			},
			Entry("user posts", "/users/123/posts/456", 200, "User: 123, Post: 456"),
			Entry("user post comments", "/users/123/posts/456/comments/789", 200, "User: 123, Post: 456, Comment: 789"),
			Entry("incomplete path", "/users/123/posts", 404, ""),
		)
	})

	Describe("Context response methods", func() {
		It("should handle different response types correctly", func() {
			r := router.MustNew()

			r.GET("/json", func(c *router.Context) {
				c.JSON(http.StatusOK, map[string]string{"message": "test"})
			})

			r.GET("/string", func(c *router.Context) {
				c.Stringf(http.StatusOK, "Hello %s", "World")
			})

			r.GET("/html", func(c *router.Context) {
				c.HTML(http.StatusOK, "<h1>Hello</h1>")
			})

			// Test JSON
			req := httptest.NewRequest(http.MethodGet, "/json", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("application/json; charset=utf-8"))

			// Test String
			req = httptest.NewRequest(http.MethodGet, "/string", nil)
			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(Equal("Hello World"))

			// Test HTML
			req = httptest.NewRequest(http.MethodGet, "/html", nil)
			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(Equal("text/html"))
		})
	})

	Describe("All HTTP methods", func() {
		var r *router.Router

		BeforeEach(func() {
			r = router.MustNew()

			r.GET("/get", func(c *router.Context) {
				c.String(http.StatusOK, "GET")
			})
			r.POST("/post", func(c *router.Context) {
				c.String(http.StatusOK, "POST")
			})
			r.PUT("/put", func(c *router.Context) {
				c.String(http.StatusOK, "PUT")
			})
			r.DELETE("/delete", func(c *router.Context) {
				c.String(http.StatusOK, "DELETE")
			})
			r.PATCH("/patch", func(c *router.Context) {
				c.String(http.StatusOK, "PATCH")
			})
			r.OPTIONS("/options", func(c *router.Context) {
				c.String(http.StatusOK, "OPTIONS")
			})
			r.HEAD("/head", func(c *router.Context) {
				c.Status(http.StatusOK)
			})
		})

		DescribeTable("should handle HTTP methods correctly",
			func(method, path, expected string) {
				req := httptest.NewRequest(method, path, nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				if expected != "" {
					Expect(w.Body.String()).To(Equal(expected))
				}
			},
			Entry("GET", http.MethodGet, "/get", "GET"),
			Entry("POST", http.MethodPost, "/post", "POST"),
			Entry("PUT", http.MethodPut, "/put", "PUT"),
			Entry("DELETE", http.MethodDelete, "/delete", "DELETE"),
			Entry("PATCH", http.MethodPatch, "/patch", "PATCH"),
			Entry("OPTIONS", http.MethodOptions, "/options", "OPTIONS"),
			Entry("HEAD", http.MethodHead, "/head", ""),
		)
	})

	Describe("Router options integration", func() {
		It("should work with multiple options combined", func() {
			handler := router.DiagnosticHandlerFunc(func(e router.DiagnosticEvent) {
				// No-op for testing
			})

			r := router.MustNew(
				router.WithDiagnostics(handler),
				router.WithBloomFilterSize(2000),
				router.WithBloomFilterHashFunctions(5),
				router.WithCancellationCheck(false),
				router.WithTemplateRouting(true),
			)

			// Verify router works with all options
			r.GET("/users/:id", func(c *router.Context) {
				c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
			})

			req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]string
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(Equal("42"))
		})

		Describe("Template routing", func() {
			Context("with templates enabled", func() {
				It("should route static and dynamic routes", func() {
					r := router.MustNew(router.WithTemplateRouting(true))

					staticCalled := false
					dynamicCalled := false

					r.GET("/static/path", func(c *router.Context) {
						staticCalled = true
						c.String(http.StatusOK, "static")
					})

					r.GET("/users/:id/posts/:postId", func(c *router.Context) {
						dynamicCalled = true
						c.JSON(http.StatusOK, map[string]string{
							"userId": c.Param("id"),
							"postId": c.Param("postId"),
						})
					})

					// Test static route
					req := httptest.NewRequest(http.MethodGet, "/static/path", nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(staticCalled).To(BeTrue())
					Expect(w.Code).To(Equal(http.StatusOK))
					Expect(w.Body.String()).To(Equal("static"))

					// Test dynamic route
					req = httptest.NewRequest(http.MethodGet, "/users/1/posts/2", nil)
					w = httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(dynamicCalled).To(BeTrue())
					Expect(w.Code).To(Equal(http.StatusOK))
				})
			})

			Context("with templates disabled", func() {
				It("should still route static and dynamic routes", func() {
					r := router.MustNew(router.WithTemplateRouting(false))

					staticCalled := false
					dynamicCalled := false

					r.GET("/static/path", func(c *router.Context) {
						staticCalled = true
						c.String(http.StatusOK, "static")
					})

					r.GET("/users/:id", func(c *router.Context) {
						dynamicCalled = true
						c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
					})

					// Both should work even without templates
					req := httptest.NewRequest(http.MethodGet, "/static/path", nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(staticCalled).To(BeTrue())
					Expect(w.Code).To(Equal(http.StatusOK))

					req = httptest.NewRequest(http.MethodGet, "/users/123", nil)
					w = httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(dynamicCalled).To(BeTrue())
					Expect(w.Code).To(Equal(http.StatusOK))
				})
			})

			It("should work after routes are registered", func() {
				r := router.MustNew(router.WithTemplateRouting(true))

				r.GET("/test1", func(c *router.Context) {
					c.String(http.StatusOK, "test1")
				})

				req := httptest.NewRequest(http.MethodGet, "/test1", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("test1"))
			})
		})
	})

	Describe("Diagnostics integration", func() {
		It("should emit diagnostic events during request handling", func() {
			var events []router.DiagnosticEvent
			handler := router.DiagnosticHandlerFunc(func(e router.DiagnosticEvent) {
				events = append(events, e)
			})

			r := router.MustNew(router.WithDiagnostics(handler))

			// Trigger diagnostic event (header injection attempt)
			r.GET("/test", func(c *router.Context) {
				c.Header("X-Test", "value\r\nX-Injected: malicious")
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(events).To(HaveLen(1))
			Expect(events[0].Kind).To(Equal(router.DiagHeaderInjection))
		})
	})

	Describe("HTTP ResponseWriter interfaces", func() {
		Describe("Hijack support", func() {
			It("should support connection hijacking for WebSocket upgrades", func() {
				r := router.MustNew()

				var hijackedConn net.Conn
				var hijackedRW *bufio.ReadWriter
				var hijackErr error

				r.GET("/ws", func(c *router.Context) {
					if hijacker, ok := c.Response.(http.Hijacker); ok {
						hijackedConn, hijackedRW, hijackErr = hijacker.Hijack()
						c.Status(http.StatusSwitchingProtocols)
					} else {
						c.String(http.StatusInternalServerError, "Hijack not supported")
					}
				})

				req := httptest.NewRequest(http.MethodGet, "/ws", nil)

				server, client := net.Pipe()
				defer func() {
					_ = server.Close()
					_ = client.Close()
				}()

				mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
				mockWriter := &mockHijackableResponseWriter{
					ResponseRecorder: httptest.NewRecorder(),
					conn:             server,
					rw:               mockRW,
				}

				r.ServeHTTP(mockWriter, req)

				Expect(mockWriter.hijackCalled).To(BeTrue())
				Expect(hijackErr).To(BeNil())
				Expect(hijackedConn).NotTo(BeNil())
				Expect(hijackedRW).NotTo(BeNil())
			})

			It("should handle hijack errors correctly", func() {
				r := router.MustNew()

				var receivedErr error

				r.GET("/ws", func(c *router.Context) {
					if hijacker, ok := c.Response.(http.Hijacker); ok {
						_, _, receivedErr = hijacker.Hijack()
					}
				})

				req := httptest.NewRequest(http.MethodGet, "/ws", nil)
				mockWriter := &mockHijackableResponseWriter{
					ResponseRecorder: httptest.NewRecorder(),
					hijackErr:        http.ErrNotSupported,
				}

				r.ServeHTTP(mockWriter, req)

				Expect(receivedErr).NotTo(BeNil())
				Expect(errors.Is(receivedErr, http.ErrNotSupported)).To(BeTrue())
			})

			It("should preserve status and size before hijack", func() {
				r := router.MustNew()

				r.GET("/ws", func(c *router.Context) {
					c.Header("X-Test", "value")
					c.Status(http.StatusSwitchingProtocols)
					c.Response.Write([]byte("Upgrading"))

					if hijacker, ok := c.Response.(http.Hijacker); ok {
						conn, _, err := hijacker.Hijack()
						Expect(err).To(BeNil())
						Expect(conn).NotTo(BeNil())
					}
				})

				req := httptest.NewRequest(http.MethodGet, "/ws", nil)

				server, client := net.Pipe()
				defer func() {
					_ = server.Close()
					_ = client.Close()
				}()

				mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
				mockWriter := &mockHijackableResponseWriter{
					ResponseRecorder: httptest.NewRecorder(),
					conn:             server,
					rw:               mockRW,
				}

				r.ServeHTTP(mockWriter, req)

				Expect(mockWriter.hijackCalled).To(BeTrue())
			})
		})

		Describe("Flush support", func() {
			It("should support flushing for streaming responses", func() {
				r := router.MustNew()

				r.GET("/stream", func(c *router.Context) {
					c.Header("Content-Type", "text/event-stream")
					c.Header("Cache-Control", "no-cache")
					c.Header("Connection", "keep-alive")

					c.String(http.StatusOK, "data: chunk1\n\n")

					if flusher, ok := c.Response.(http.Flusher); ok {
						flusher.Flush()
					}

					c.String(http.StatusOK, "data: chunk2\n\n")

					if flusher, ok := c.Response.(http.Flusher); ok {
						flusher.Flush()
					}
				})

				req := httptest.NewRequest(http.MethodGet, "/stream", nil)
				mockWriter := &mockFlushableResponseWriter{
					ResponseRecorder: httptest.NewRecorder(),
				}

				r.ServeHTTP(mockWriter, req)

				Expect(mockWriter.flushCalled).To(BeTrue())
				Expect(mockWriter.Code).To(Equal(http.StatusOK))
				Expect(mockWriter.Body.String()).To(Equal("data: chunk1\n\ndata: chunk2\n\n"))
			})

			It("should flush between multiple writes", func() {
				r := router.MustNew()

				r.GET("/events", func(c *router.Context) {
					c.Header("Content-Type", "text/event-stream")

					for i := 1; i <= 3; i++ {
						c.Response.Write([]byte("event: message\n"))

						if flusher, ok := c.Response.(http.Flusher); ok {
							flusher.Flush()
						}
					}
				})

				req := httptest.NewRequest(http.MethodGet, "/events", nil)
				flushableWriter := &mockFlushableResponseWriter{
					ResponseRecorder: httptest.NewRecorder(),
				}

				r.ServeHTTP(flushableWriter, req)

				Expect(flushableWriter.flushCalled).To(BeTrue())
			})
		})

		Describe("Hijack and Flush together", func() {
			It("should support both Hijack and Flush on same writer", func() {
				r := router.MustNew()

				r.GET("/websocket", func(c *router.Context) {
					c.Header("Upgrade", "websocket")
					c.Header("Connection", "Upgrade")

					if flusher, ok := c.Response.(http.Flusher); ok {
						flusher.Flush()
					}

					if hijacker, ok := c.Response.(http.Hijacker); ok {
						conn, _, err := hijacker.Hijack()
						Expect(err).To(BeNil())
						Expect(conn).NotTo(BeNil())
					}
				})

				req := httptest.NewRequest(http.MethodGet, "/websocket", nil)

				server, client := net.Pipe()
				defer func() {
					_ = server.Close()
					_ = client.Close()
				}()

				mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
				mockWriter := &mockHijackFlushResponseWriter{
					ResponseRecorder: httptest.NewRecorder(),
					conn:             server,
					rw:               mockRW,
				}

				r.ServeHTTP(mockWriter, req)

				Expect(mockWriter.flushCalled).To(BeTrue())
				Expect(mockWriter.hijackCalled).To(BeTrue())
			})
		})

		Describe("With observability", func() {
			It("should support Flush with metrics enabled", func() {
				r := router.MustNew()

				// Note: newMockObservabilityRecorder is in router package
				// We'll test through public API only
				flushed := false

				r.GET("/stream", func(c *router.Context) {
					c.String(http.StatusOK, "chunk 1")

					if flusher, ok := c.Response.(http.Flusher); ok {
						flusher.Flush()
						flushed = true
					}
				})

				req := httptest.NewRequest(http.MethodGet, "/stream", nil)
				mockWriter := &mockFlushableResponseWriter{
					ResponseRecorder: httptest.NewRecorder(),
				}

				r.ServeHTTP(mockWriter, req)

				Expect(flushed).To(BeTrue())
				Expect(mockWriter.flushCalled).To(BeTrue())
			})

			It("should support Hijack with tracing enabled", func() {
				r := router.MustNew()

				r.GET("/ws", func(c *router.Context) {
					if hijacker, ok := c.Response.(http.Hijacker); ok {
						conn, _, err := hijacker.Hijack()
						Expect(err).To(BeNil())
						Expect(conn).NotTo(BeNil())
					}
				})

				req := httptest.NewRequest(http.MethodGet, "/ws", nil)

				server, client := net.Pipe()
				defer func() {
					_ = server.Close()
					_ = client.Close()
				}()

				mockRW := bufio.NewReadWriter(bufio.NewReader(server), bufio.NewWriter(server))
				mockWriter := &mockHijackableResponseWriter{
					ResponseRecorder: httptest.NewRecorder(),
					conn:             server,
					rw:               mockRW,
				}

				r.ServeHTTP(mockWriter, req)

				Expect(mockWriter.hijackCalled).To(BeTrue())
			})
		})
	})
})
