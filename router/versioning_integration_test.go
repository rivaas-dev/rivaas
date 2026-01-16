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

//go:build integration

package router_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"rivaas.dev/router"
	"rivaas.dev/router/version"
)

var _ = Describe("Versioning Integration", func() {
	Describe("Explicit versioning precedence", func() {
		// This tests the core design principle: routes are only versioned if explicitly
		// registered via r.GET() etc. bypass version
		// detection and always take precedence.

		Describe("Main tree takes precedence over version trees", func() {
			var r *router.Router

			BeforeEach(func() {
				r = router.MustNew(router.WithVersioning(
					version.WithHeaderDetection("X-API-Version"),
					version.WithDefault("v1"),
					version.WithValidVersions("v1", "v2"),
				))

				// Non-versioned routes (main tree) - always accessible
				r.GET("/health", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "healthy")).NotTo(HaveOccurred())
				})
				r.GET("/metrics", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "metrics")).NotTo(HaveOccurred())
				})
				r.GET("/users/:id", func(c *router.Context) {
					Expect(c.Stringf(http.StatusOK, "main-tree-user-%s", c.Param("id"))).NotTo(HaveOccurred())
				})

				// Versioned routes (version trees) - subject to version detection
				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})

				v2 := r.Version("v2")
				v2.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 users")).NotTo(HaveOccurred())
				})
			})

			DescribeTable("non-versioned routes bypass version detection",
				func(path, header, expected string) {
					req := httptest.NewRequest(http.MethodGet, path, nil)
					if header != "" {
						req.Header.Set("X-Api-Version", header)
					}
					w := httptest.NewRecorder()

					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(http.StatusOK))
					Expect(w.Body.String()).To(Equal(expected))
				},
				// /health is non-versioned - always returns "healthy" regardless of version header
				Entry("health with no header", "/health", "", "healthy"),
				Entry("health with v1 header", "/health", "v1", "healthy"),
				Entry("health with v2 header", "/health", "v2", "healthy"),
				Entry("health with invalid version", "/health", "v99", "healthy"),

				// /metrics is non-versioned
				Entry("metrics with no header", "/metrics", "", "metrics"),
				Entry("metrics with v2 header", "/metrics", "v2", "metrics"),

				// /users/:id is in main tree - takes precedence
				Entry("users/:id with no header", "/users/123", "", "main-tree-user-123"),
				Entry("users/:id with v1 header", "/users/456", "v1", "main-tree-user-456"),
				Entry("users/:id with v2 header", "/users/789", "v2", "main-tree-user-789"),
			)

			DescribeTable("versioned routes respect version detection",
				func(path, header, expected string) {
					req := httptest.NewRequest(http.MethodGet, path, nil)
					if header != "" {
						req.Header.Set("X-Api-Version", header)
					}
					w := httptest.NewRecorder()

					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(http.StatusOK))
					Expect(w.Body.String()).To(Equal(expected))
				},
				// /users (without :id) is only in version trees
				Entry("users with no header defaults to v1", "/users", "", "v1 users"),
				Entry("users with v1 header", "/users", "v1", "v1 users"),
				Entry("users with v2 header", "/users", "v2", "v2 users"),
				Entry("users with invalid version defaults to v1", "/users", "v99", "v1 users"),
			)
		})

		Describe("Mixed routes with path versioning", func() {
			var r *router.Router

			BeforeEach(func() {
				r = router.MustNew(router.WithVersioning(
					version.WithPathDetection("/v{version}/"),
					version.WithDefault("v1"),
					version.WithValidVersions("v1", "v2"),
				))

				// Non-versioned routes - always accessible at the exact path
				r.GET("/health", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "healthy")).NotTo(HaveOccurred())
				})

				// Versioned routes
				v1 := r.Version("v1")
				v1.GET("/data", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 data")).NotTo(HaveOccurred())
				})

				v2 := r.Version("v2")
				v2.GET("/data", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 data")).NotTo(HaveOccurred())
				})
			})

			It("non-versioned route is accessible without path version", func() {
				req := httptest.NewRequest(http.MethodGet, "/health", nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("healthy"))
			})

			It("versioned routes work with path versioning", func() {
				req := httptest.NewRequest(http.MethodGet, "/v2/data", nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("v2 data"))
			})
		})

		Describe("No version context for non-versioned routes", func() {
			var r *router.Router

			BeforeEach(func() {
				r = router.MustNew(router.WithVersioning(
					version.WithHeaderDetection("X-API-Version"),
					version.WithDefault("v1"),
				))

				// Non-versioned route - version should be empty
				r.GET("/health", func(c *router.Context) {
					ver := c.Version()
					if ver == "" {
						Expect(c.String(http.StatusOK, "no-version")).NotTo(HaveOccurred())
					} else {
						Expect(c.Stringf(http.StatusOK, "version-%s", ver)).NotTo(HaveOccurred())
					}
				})

				// Versioned route - version should be set
				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.Stringf(http.StatusOK, "version-%s", c.Version())).NotTo(HaveOccurred())
				})
			})

			It("non-versioned route has empty version", func() {
				req := httptest.NewRequest(http.MethodGet, "/health", nil)
				req.Header.Set("X-Api-Version", "v1") // Should be ignored
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("no-version"))
			})

			It("versioned route has version set", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("X-Api-Version", "v1")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("version-v1"))
			})
		})
	})

	Describe("Header-based versioning", func() {
		var r *router.Router

		BeforeEach(func() {
			r = router.MustNew(router.WithVersioning(
				version.WithHeaderDetection("X-API-Version"),
				version.WithDefault("v1"),
				version.WithValidVersions("v1", "v2"),
			))
		})

		Describe("Versioned routing", func() {
			BeforeEach(func() {
				// Add v1 routes - using static routes for PUT/DELETE/PATCH to ensure they're tested
				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})
				v1.POST("/users", func(c *router.Context) {
					Expect(c.String(http.StatusCreated, "v1 user created")).NotTo(HaveOccurred())
				})
				v1.PUT("/users/123", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 updated user 123")).NotTo(HaveOccurred())
				})
				v1.DELETE("/users/456", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 deleted user 456")).NotTo(HaveOccurred())
				})
				v1.PATCH("/users/789", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 patched user 789")).NotTo(HaveOccurred())
				})
				v1.OPTIONS("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 options")).NotTo(HaveOccurred())
				})
				v1.HEAD("/users", func(c *router.Context) {
					c.Status(http.StatusOK)
				})

				// Add v2 routes
				v2 := r.Version("v2")
				v2.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 users")).NotTo(HaveOccurred())
				})
				v2.POST("/users", func(c *router.Context) {
					Expect(c.String(http.StatusCreated, "v2 user created")).NotTo(HaveOccurred())
				})
			})

			DescribeTable("HTTP methods with version header",
				func(method, path, ver, expected string, status int) {
					req := httptest.NewRequest(method, path, nil)
					req.Header.Set("X-Api-Version", ver)
					w := httptest.NewRecorder()

					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(status))
					if expected != "" {
						Expect(w.Body.String()).To(Equal(expected))
					}
				},
				Entry("v1 GET", http.MethodGet, "/users", "v1", "v1 users", http.StatusOK),
				Entry("v2 GET", http.MethodGet, "/users", "v2", "v2 users", http.StatusOK),
				Entry("v1 POST", http.MethodPost, "/users", "v1", "v1 user created", http.StatusCreated),
				Entry("v2 POST", http.MethodPost, "/users", "v2", "v2 user created", http.StatusCreated),
				Entry("v1 PUT", http.MethodPut, "/users/123", "v1", "v1 updated user 123", http.StatusOK),
				Entry("v1 DELETE", http.MethodDelete, "/users/456", "v1", "v1 deleted user 456", http.StatusOK),
				Entry("v1 PATCH", http.MethodPatch, "/users/789", "v1", "v1 patched user 789", http.StatusOK),
				Entry("v1 OPTIONS", http.MethodOptions, "/users", "v1", "v1 options", http.StatusOK),
				Entry("v1 HEAD", http.MethodHead, "/users", "v1", "", http.StatusOK),
			)
		})

		Describe("Versioned groups", func() {
			BeforeEach(func() {
				// Create versioned groups - using static paths to ensure they work
				v1 := r.Version("v1")
				v1Group := v1.Group("/api")
				v1Group.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 api users")).NotTo(HaveOccurred())
				})
				v1Group.POST("/users", func(c *router.Context) {
					Expect(c.String(http.StatusCreated, "v1 api user created")).NotTo(HaveOccurred())
				})
				v1Group.PUT("/users/123", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 api updated 123")).NotTo(HaveOccurred())
				})
				v1Group.DELETE("/users/456", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 api deleted 456")).NotTo(HaveOccurred())
				})
				v1Group.PATCH("/users/789", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 api patched 789")).NotTo(HaveOccurred())
				})
				v1Group.OPTIONS("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 api options")).NotTo(HaveOccurred())
				})
				v1Group.HEAD("/users", func(c *router.Context) {
					c.Status(http.StatusOK)
				})
			})

			DescribeTable("HTTP methods with versioned groups",
				func(method, path, expected string, status int) {
					req := httptest.NewRequest(method, path, nil)
					req.Header.Set("X-Api-Version", "v1")
					w := httptest.NewRecorder()

					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(status))
					if expected != "" {
						Expect(w.Body.String()).To(Equal(expected))
					}
				},
				Entry("GET", http.MethodGet, "/api/users", "v1 api users", http.StatusOK),
				Entry("POST", http.MethodPost, "/api/users", "v1 api user created", http.StatusCreated),
				Entry("PUT", http.MethodPut, "/api/users/123", "v1 api updated 123", http.StatusOK),
				Entry("DELETE", http.MethodDelete, "/api/users/456", "v1 api deleted 456", http.StatusOK),
				Entry("PATCH", http.MethodPatch, "/api/users/789", "v1 api patched 789", http.StatusOK),
				Entry("OPTIONS", http.MethodOptions, "/api/users", "v1 api options", http.StatusOK),
				Entry("HEAD", http.MethodHead, "/api/users", "", http.StatusOK),
			)
		})

		Describe("Versioned routing with compilation", func() {
			BeforeEach(func() {
				v1 := r.Version("v1")
				v1.GET("/static1", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 static1")).NotTo(HaveOccurred())
				})
				v1.GET("/static2", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 static2")).NotTo(HaveOccurred())
				})

				// Compile routes
				r.Warmup()
			})

			It("routes to compiled versioned routes", func() {
				req := httptest.NewRequest(http.MethodGet, "/static1", nil)
				req.Header.Set("X-Api-Version", "v1")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("v1 static1"))
			})
		})
	})

	Describe("Query parameter versioning", func() {
		var r *router.Router

		BeforeEach(func() {
			r = router.MustNew(router.WithVersioning(
				version.WithQueryDetection("version"),
				version.WithDefault("v1"),
				version.WithValidVersions("v1", "v2"),
			))

			v1 := r.Version("v1")
			v1.GET("/data", func(c *router.Context) {
				Expect(c.String(http.StatusOK, "v1 data")).NotTo(HaveOccurred())
			})

			v2 := r.Version("v2")
			v2.GET("/data", func(c *router.Context) {
				Expect(c.String(http.StatusOK, "v2 data")).NotTo(HaveOccurred())
			})
		})

		DescribeTable("version selection from query parameter",
			func(url, expected string) {
				req := httptest.NewRequest(http.MethodGet, url, nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal(expected))
			},
			Entry("default version when no query param", "/data", "v1 data"),
			Entry("v1 explicit", "/data?version=v1", "v1 data"),
			Entry("v2 explicit", "/data?version=v2", "v2 data"),
			Entry("invalid version defaults to v1", "/data?version=v3", "v1 data"),
		)
	})

	Describe("Custom version detector", func() {
		var r *router.Router

		BeforeEach(func() {
			r = router.MustNew(router.WithVersioning(
				version.WithCustomDetection(func(req *http.Request) string {
					// Custom logic: extract version from user-agent
					ua := req.UserAgent()
					if ua == "ClientV2" {
						return "v2"
					}

					return "v1"
				}),
				version.WithDefault("v1"),
			))

			v1 := r.Version("v1")
			v1.GET("/data", func(c *router.Context) {
				Expect(c.String(http.StatusOK, "v1 data")).NotTo(HaveOccurred())
			})

			v2 := r.Version("v2")
			v2.GET("/data", func(c *router.Context) {
				Expect(c.String(http.StatusOK, "v2 data")).NotTo(HaveOccurred())
			})
		})

		It("routes to v1 with ClientV1 user agent", func() {
			req := httptest.NewRequest(http.MethodGet, "/data", nil)
			req.Header.Set("User-Agent", "ClientV1")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			Expect(w.Body.String()).To(Equal("v1 data"))
		})

		It("routes to v2 with ClientV2 user agent", func() {
			req := httptest.NewRequest(http.MethodGet, "/data", nil)
			req.Header.Set("User-Agent", "ClientV2")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			Expect(w.Body.String()).To(Equal("v2 data"))
		})
	})

	Describe("Path-based versioning", func() {
		Describe("Basic path versioning", func() {
			var r *router.Router

			BeforeEach(func() {
				r = router.MustNew(router.WithVersioning(
					version.WithPathDetection("/v{version}/"),
					version.WithDefault("v1"),
					version.WithValidVersions("v1", "v2", "v3"),
				))

				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})
				v1.GET("/posts", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 posts")).NotTo(HaveOccurred())
				})

				v2 := r.Version("v2")
				v2.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 users")).NotTo(HaveOccurred())
				})
				v2.GET("/posts", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 posts")).NotTo(HaveOccurred())
				})
			})

			DescribeTable("path version selection",
				func(path, expected string, status int) {
					req := httptest.NewRequest(http.MethodGet, path, nil)
					w := httptest.NewRecorder()

					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(status))
					Expect(w.Body.String()).To(Equal(expected))
				},
				Entry("v1 users", "/v1/users", "v1 users", http.StatusOK),
				Entry("v2 users", "/v2/users", "v2 users", http.StatusOK),
				Entry("v1 posts", "/v1/posts", "v1 posts", http.StatusOK),
				Entry("v2 posts", "/v2/posts", "v2 posts", http.StatusOK),
				Entry("default when no version", "/users", "v1 users", http.StatusOK),
				Entry("invalid version defaults", "/v99/users", "v1 users", http.StatusOK),
			)
		})

		Describe("Path with API prefix", func() {
			var r *router.Router

			BeforeEach(func() {
				r = router.MustNew(router.WithVersioning(
					version.WithPathDetection("/api/v{version}/"),
					version.WithDefault("v1"),
				))

				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 api users")).NotTo(HaveOccurred())
				})

				v2 := r.Version("v2")
				v2.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 api users")).NotTo(HaveOccurred())
				})
			})

			DescribeTable("path versioning with API prefix",
				func(path, expected string, status int) {
					req := httptest.NewRequest(http.MethodGet, path, nil)
					w := httptest.NewRecorder()

					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(status))
					Expect(w.Body.String()).To(Equal(expected))
				},
				Entry("v1 with api prefix", "/api/v1/users", "v1 api users", http.StatusOK),
				Entry("v2 with api prefix", "/api/v2/users", "v2 api users", http.StatusOK),
			)
		})

		Describe("Version detection priority", func() {
			var r *router.Router

			BeforeEach(func() {
				r = router.MustNew(router.WithVersioning(
					version.WithPathDetection("/v{version}/"),
					version.WithHeaderDetection("X-API-Version"),
					version.WithQueryDetection("version"),
					version.WithDefault("v1"),
				))

				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})

				v2 := r.Version("v2")
				v2.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 users")).NotTo(HaveOccurred())
				})

				v3 := r.Version("v3")
				v3.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v3 users")).NotTo(HaveOccurred())
				})
			})

			DescribeTable("path priority over other methods",
				func(path, header, expected string) {
					req := httptest.NewRequest(http.MethodGet, path, nil)
					if header != "" {
						req.Header.Set("X-Api-Version", header)
					}
					w := httptest.NewRecorder()

					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(http.StatusOK))
					Expect(w.Body.String()).To(Equal(expected))
				},
				Entry("path overrides header", "/v2/users", "v3", "v2 users"),
				Entry("path overrides query", "/v2/users?version=v3", "", "v2 users"),
				Entry("path overrides both header and query", "/v2/users?version=v1", "v3", "v2 users"),
			)
		})

		Describe("Path versioning with validation", func() {
			var r *router.Router

			BeforeEach(func() {
				r = router.MustNew(router.WithVersioning(
					version.WithPathDetection("/v{version}/"),
					version.WithValidVersions("v1", "v2"),
					version.WithDefault("v1"),
				))

				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})

				v2 := r.Version("v2")
				v2.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 users")).NotTo(HaveOccurred())
				})
			})

			DescribeTable("path versioning with validation",
				func(path, expected string, status int) {
					req := httptest.NewRequest(http.MethodGet, path, nil)
					w := httptest.NewRecorder()

					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(status))
					Expect(w.Body.String()).To(Equal(expected))
				},
				Entry("valid v1", "/v1/users", "v1 users", http.StatusOK),
				Entry("valid v2", "/v2/users", "v2 users", http.StatusOK),
				Entry("invalid v3 defaults", "/v3/users", "v1 users", http.StatusOK),
				Entry("invalid v99 defaults", "/v99/users", "v1 users", http.StatusOK),
				Entry("no version defaults", "/users", "v1 users", http.StatusOK),
			)
		})

		Describe("Path versioned groups", func() {
			var r *router.Router

			BeforeEach(func() {
				r = router.MustNew(router.WithVersioning(
					version.WithPathDetection("/v{version}/"),
					version.WithDefault("v1"),
				))

				v1 := r.Version("v1")
				v1Group := v1.Group("/api")
				v1Group.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 api users")).NotTo(HaveOccurred())
				})

				v2 := r.Version("v2")
				v2Group := v2.Group("/api")
				v2Group.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 api users")).NotTo(HaveOccurred())
				})
			})

			DescribeTable("path versioned groups",
				func(path, expected string, status int) {
					req := httptest.NewRequest(http.MethodGet, path, nil)
					w := httptest.NewRecorder()

					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(status))
					Expect(w.Body.String()).To(Equal(expected))
				},
				Entry("v1 api group", "/v1/api/users", "v1 api users", http.StatusOK),
				Entry("v2 api group", "/v2/api/users", "v2 api users", http.StatusOK),
			)
		})

		Describe("All HTTP methods", func() {
			var r *router.Router

			BeforeEach(func() {
				r = router.MustNew(router.WithVersioning(
					version.WithPathDetection("/v{version}/"),
					version.WithDefault("v1"),
				))

				v1 := r.Version("v1")
				v1.GET("/resource", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 GET")).NotTo(HaveOccurred())
				})
				v1.POST("/resource", func(c *router.Context) {
					Expect(c.String(http.StatusCreated, "v1 POST")).NotTo(HaveOccurred())
				})
				v1.PUT("/resource/123", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 PUT")).NotTo(HaveOccurred())
				})
				v1.DELETE("/resource/456", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 DELETE")).NotTo(HaveOccurred())
				})
				v1.PATCH("/resource/789", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 PATCH")).NotTo(HaveOccurred())
				})
				v1.OPTIONS("/resource", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 OPTIONS")).NotTo(HaveOccurred())
				})
				v1.HEAD("/resource", func(c *router.Context) {
					c.Status(http.StatusOK)
				})
			})

			DescribeTable("all HTTP methods with path versioning",
				func(method, path, expected string, status int) {
					req := httptest.NewRequest(method, path, nil)
					w := httptest.NewRecorder()

					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(status))
					if expected != "" {
						Expect(w.Body.String()).To(Equal(expected))
					}
				},
				Entry("GET", "GET", "/v1/resource", "v1 GET", http.StatusOK),
				Entry("POST", "POST", "/v1/resource", "v1 POST", http.StatusCreated),
				Entry("PUT", "PUT", "/v1/resource/123", "v1 PUT", http.StatusOK),
				Entry("DELETE", "DELETE", "/v1/resource/456", "v1 DELETE", http.StatusOK),
				Entry("PATCH", "PATCH", "/v1/resource/789", "v1 PATCH", http.StatusOK),
				Entry("OPTIONS", "OPTIONS", "/v1/resource", "v1 OPTIONS", http.StatusOK),
				Entry("HEAD", "HEAD", "/v1/resource", "", http.StatusOK),
			)
		})

		Describe("Path stripping edge cases", func() {
			It("handles no path-based versioning", func() {
				// Test: No path-based versioning or no version detected
				// Router without path versioning enabled - PathEnabled will be false
				r := router.MustNew(router.WithVersioning(
					version.WithHeaderDetection("X-API-Version"), // Only header versioning
					version.WithDefault("v1"),
				))

				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "users")).NotTo(HaveOccurred())
				})

				// Path versioning isn't enabled, so the path should remain unchanged
				// Route registered as "/users" should match "/users" (not "/v1/users")
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("X-Api-Version", "v1")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				// Should match route (versioning works via header, path not modified)
				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("users"))
			})

			It("handles no version detected", func() {
				// Test: An empty version detected (version == "")
				r := router.MustNew(router.WithVersioning(
					version.WithPathDetection("/v{version}/"),
					version.WithDefault("v1"),
				))

				// Register a route without a version prefix
				r.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "users")).NotTo(HaveOccurred())
				})

				// Request with no version in the path
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("handles prefix extending beyond path", func() {
				// Test: Invalid case where the prefix extends beyond path
				// This tests the condition where versionStart >= len(path)
				r := router.MustNew(router.WithVersioning(
					version.WithPathDetection("/very/long/prefix/v{version}/"),
					version.WithDefault("v1"),
				))

				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})

				// Request with a path that exactly matches the prefix (no version segment)
				// This should trigger the condition where prefix length >= path length
				req := httptest.NewRequest(http.MethodGet, "/very/long/prefix/v", nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				// Should still attempt to process (a path doesn't match any route)
				// The stripPathVersion returns the path unchanged in this case
				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("handles version at end of path", func() {
				// Test: Version is at the end of the path (e.g., "/v1")
				// This also tests: Version at ends, strips everything, returns "/"
				r := router.MustNew(router.WithVersioning(
					version.WithPathDetection("/v{version}/"),
					version.WithDefault("v1"),
				))

				// Register a route at root
				v1 := r.Version("v1")
				v1.GET("/", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "root")).NotTo(HaveOccurred())
				})

				// Request with a version at the end: "/v1"
				req := httptest.NewRequest(http.MethodGet, "/v1", nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				// Should strip to "/" and match the root route
				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("root"))
			})

			It("handles version that doesn't match", func() {
				// Test: Version doesn't match, don't strip
				// This tests the condition where a version segment doesn't match a detected version
				r := router.MustNew(router.WithVersioning(
					version.WithPathDetection("/v{version}/"),
					version.WithDefault("v1"),
				))

				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})

				// Request with a path "/v2/users" but the detected version is "v1"
				// This happens when version detection fails but path has a different version
				// The stripPathVersion will check if a version matches, and if not, return a path unchanged
				req := httptest.NewRequest(http.MethodGet, "/v2/users", nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				// Since v2 is not valid, should default to v1,
				// But the path stripping logic may still be involved.
				// Let's verify the behavior - should use default version v1
				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("v1 users"))
			})

			It("handles path becoming root after stripping", func() {
				// Test: Path becomes root after stripping
				// This tests the condition where strippedStart >= len(path)
				r := router.MustNew(router.WithVersioning(
					version.WithPathDetection("/api/v{version}/"),
					version.WithDefault("v1"),
				))

				v1 := r.Version("v1")
				v1.GET("/", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "root")).NotTo(HaveOccurred())
				})

				// Request: "/api/v1/" - after stripping prefix "/api/v" and version "1",
				// we should get "/" (root)
				req := httptest.NewRequest(http.MethodGet, "/api/v1/", nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				// Should match the root route
				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("root"))
			})

			It("handles version at end with trailing slash", func() {
				// Additional test: version at end with trailing slash
				// This also tests: Version at end, strip everything, return "/"
				r := router.MustNew(router.WithVersioning(
					version.WithPathDetection("/v{version}/"),
					version.WithDefault("v1"),
				))

				v1 := r.Version("v1")
				v1.GET("/", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "root")).NotTo(HaveOccurred())
				})

				// Request "/v1/" - version at end (after trailing slash handling)
				req := httptest.NewRequest(http.MethodGet, "/v1/", nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				// Should strip to "/" and match root
				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("root"))
			})
		})
	})

	Describe("Accept header versioning", func() {
		Describe("Basic accept versioning", func() {
			var r *router.Router

			BeforeEach(func() {
				r = router.MustNew(router.WithVersioning(
					version.WithAcceptDetection("application/vnd.myapi.{version}+json"),
					version.WithDefault("v1"),
				))

				v2 := r.Version("v2")
				v2.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 users")).NotTo(HaveOccurred())
				})
			})

			It("selects version from Accept header", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("Accept", "application/vnd.myapi.v2+json")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("v2 users"))
			})
		})

		Describe("Accept with multiple media types", func() {
			var r *router.Router

			BeforeEach(func() {
				r = router.MustNew(router.WithVersioning(
					version.WithAcceptDetection("application/vnd.myapi.{version}+json"),
					version.WithDefault("v1"),
				))

				v3 := r.Version("v3")
				v3.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v3 users")).NotTo(HaveOccurred())
				})
			})

			It("selects version from multiple media types", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("Accept", "text/html, application/json, application/vnd.myapi.v3+json, */*")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("v3 users"))
			})
		})
	})

	Describe("Deprecation with lifecycle options", func() {
		Describe("Deprecated version headers", func() {
			var r *router.Router
			var sunsetTime time.Time

			BeforeEach(func() {
				sunsetTime = time.Now().Add(30 * 24 * time.Hour)
				r = router.MustNew(router.WithVersioning(
					version.WithHeaderDetection("X-API-Version"),
					version.WithDefault("v1"),
				))

				// Use lifecycle options on the version
				v1 := r.Version("v1",
					version.Deprecated(),
					version.Sunset(sunsetTime),
				)
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})
			})

			It("includes deprecation headers", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("X-Api-Version", "v1")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Header().Get("Deprecation")).To(Equal("true"))
				Expect(w.Header().Get("Sunset")).ToNot(BeEmpty())
			})
		})

		Describe("Non-deprecated version", func() {
			var r *router.Router
			var sunsetTime time.Time

			BeforeEach(func() {
				sunsetTime = time.Now().Add(30 * 24 * time.Hour)
				r = router.MustNew(router.WithVersioning(
					version.WithHeaderDetection("X-API-Version"),
					version.WithDefault("v1"),
				))

				// v1 is deprecated
				v1 := r.Version("v1",
					version.Deprecated(),
					version.Sunset(sunsetTime),
				)
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})

				// v2 is NOT deprecated
				v2 := r.Version("v2")
				v2.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 users")).NotTo(HaveOccurred())
				})
			})

			It("does not include deprecation headers", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("X-Api-Version", "v2")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Header().Get("Deprecation")).To(BeEmpty())
				Expect(w.Header().Get("Sunset")).To(BeEmpty())
			})
		})

		Describe("Migration docs", func() {
			var r *router.Router

			BeforeEach(func() {
				sunsetTime := time.Now().Add(30 * 24 * time.Hour)
				r = router.MustNew(router.WithVersioning(
					version.WithHeaderDetection("X-API-Version"),
					version.WithDefault("v1"),
				))

				v1 := r.Version("v1",
					version.Deprecated(),
					version.Sunset(sunsetTime),
					version.MigrationDocs("https://docs.example.com/migrate/v1-to-v2"),
				)
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})
			})

			It("includes Link header with migration docs", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("X-Api-Version", "v1")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				linkHeader := w.Header().Get("Link")
				Expect(linkHeader).To(ContainSubstring("https://docs.example.com/migrate/v1-to-v2"))
				Expect(linkHeader).To(ContainSubstring("rel=\"deprecation\""))
			})
		})

		Describe("Configure method", func() {
			var r *router.Router

			BeforeEach(func() {
				sunsetTime := time.Now().Add(30 * 24 * time.Hour)
				r = router.MustNew(router.WithVersioning(
					version.WithHeaderDetection("X-API-Version"),
					version.WithDefault("v1"),
				))

				// Create a version first, then configure
				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})

				// Configure lifecycle later
				v1.Configure(
					version.Deprecated(),
					version.Sunset(sunsetTime),
				)
			})

			It("applies lifecycle configuration", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("X-Api-Version", "v1")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Header().Get("Deprecation")).To(Equal("true"))
				Expect(w.Header().Get("Sunset")).ToNot(BeEmpty())
			})
		})
	})

	Describe("Observability hooks", func() {
		Describe("Version detected callback", func() {
			var r *router.Router
			var detectedVersions []string
			var detectedMethods []string

			BeforeEach(func() {
				detectedVersions = []string{}
				detectedMethods = []string{}

				r = router.MustNew(router.WithVersioning(
					version.WithHeaderDetection("X-API-Version"),
					version.WithDefault("v1"),
					version.WithObserver(
						version.OnDetected(func(ver, method string) {
							detectedVersions = append(detectedVersions, ver)
							detectedMethods = append(detectedMethods, method)
						}),
					),
				))

				v2 := r.Version("v2")
				v2.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 users")).NotTo(HaveOccurred())
				})
			})

			It("calls callback on version detection", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("X-Api-Version", "v2")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(detectedVersions).To(ContainElement("v2"))
				Expect(detectedMethods).To(ContainElement("header"))
			})
		})

		Describe("Version missing callback", func() {
			var r *router.Router
			var missingCount int

			BeforeEach(func() {
				missingCount = 0

				r = router.MustNew(router.WithVersioning(
					version.WithHeaderDetection("X-API-Version"),
					version.WithDefault("v1"),
					version.WithObserver(
						version.OnMissing(func() {
							missingCount++
						}),
					),
				))

				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})
			})

			It("calls the callback when version is missing", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				// No X-API-Version header set
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("v1 users")) // Should use default
				Expect(missingCount).To(Equal(1))
			})
		})

		Describe("Invalid version callback", func() {
			var r *router.Router
			var invalidVersions []string

			BeforeEach(func() {
				invalidVersions = []string{}

				r = router.MustNew(router.WithVersioning(
					version.WithHeaderDetection("X-API-Version"),
					version.WithValidVersions("v1", "v2"),
					version.WithDefault("v1"),
					version.WithObserver(
						version.OnInvalid(func(attempted string) {
							invalidVersions = append(invalidVersions, attempted)
						}),
					),
				))

				v1 := r.Version("v1")
				v1.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v1 users")).NotTo(HaveOccurred())
				})
			})

			It("calls the callback on invalid version", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("X-Api-Version", "v99") // Invalid version
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal("v1 users")) // Should use default
				Expect(invalidVersions).To(ContainElement("v99"))
			})
		})

		Describe("Multiple detection methods", func() {
			var r *router.Router
			var methods []string

			BeforeEach(func() {
				methods = []string{}

				r = router.MustNew(router.WithVersioning(
					version.WithPathDetection("/v{version}/"),
					version.WithHeaderDetection("X-API-Version"),
					version.WithAcceptDetection("application/vnd.api.{version}+json"),
					version.WithQueryDetection("v"),
					version.WithDefault("v1"),
					version.WithObserver(
						version.OnDetected(func(_, method string) {
							methods = append(methods, method)
						}),
					),
				))

				v2 := r.Version("v2")
				v2.GET("/users", func(c *router.Context) {
					Expect(c.String(http.StatusOK, "v2 users")).NotTo(HaveOccurred())
				})
			})

			It("detects path version", func() {
				req := httptest.NewRequest(http.MethodGet, "/v2/users", nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(methods).To(ContainElement("path"))
			})

			It("detects header version", func() {
				methods = []string{}
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("X-Api-Version", "v2")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(methods).To(ContainElement("header"))
			})

			It("detects accept version", func() {
				methods = []string{}
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("Accept", "application/vnd.api.v2+json")
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(methods).To(ContainElement("accept"))
			})

			It("detects query version", func() {
				methods = []string{}
				req := httptest.NewRequest(http.MethodGet, "/users?v=v2", nil)
				w := httptest.NewRecorder()

				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(methods).To(ContainElement("query"))
			})
		})
	})

	Describe("Complex Versioning Scenarios", func() {
		var (
			r                *router.Router
			sunsetV1         time.Time
			detectedVersions []string
			invalidVersions  []string
		)

		BeforeEach(func() {
			// Reset a shared state for each test
			detectedVersions = []string{}
			invalidVersions = []string{}
			sunsetV1 = time.Now().Add(30 * 24 * time.Hour)

			// Set up a router with all versioning strategies
			r = router.MustNew(router.WithVersioning(
				version.WithPathDetection("/v{version}/"),
				version.WithHeaderDetection("X-API-Version"),
				version.WithAcceptDetection("application/vnd.api.{version}+json"),
				version.WithQueryDetection("v"),
				version.WithValidVersions("v1", "v2", "v3"),
				version.WithDefault("v1"),
				version.WithObserver(
					version.OnDetected(func(ver, _ string) {
						detectedVersions = append(detectedVersions, ver)
					}),
					version.OnInvalid(func(attempted string) {
						invalidVersions = append(invalidVersions, attempted)
					}),
				),
			))

			// Set up versioned routes for all versions
			// v1 is deprecated
			v1 := r.Version("v1",
				version.Deprecated(),
				version.Sunset(sunsetV1),
			)
			v1.GET("/users", func(c *router.Context) {
				Expect(c.Stringf(http.StatusOK, `version: %s`, c.Version())).NotTo(HaveOccurred())
			})

			// v2 and v3 are not deprecated
			for _, ver := range []string{"v2", "v3"} {
				version := ver
				vr := r.Version(version)
				vr.GET("/users", func(c *router.Context) {
					Expect(c.Stringf(http.StatusOK, `version: %s`, c.Version())).NotTo(HaveOccurred())
				})
			}
		})

		Describe("Path-based versioning", func() {
			Context("with a deprecated version", func() {
				It("routes to the correct version handler", func() {
					req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(http.StatusOK))
					Expect(w.Body.String()).To(Equal(`version: v1`))
				})

				It("includes deprecation headers", func() {
					req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(w.Header().Get("Deprecation")).To(Equal("true"))
					Expect(w.Header().Get("Sunset")).ToNot(BeEmpty())
				})

				It("records version detection", func() {
					req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(detectedVersions).To(ContainElement("v1"))
				})
			})

			Context("with non-deprecated version", func() {
				It("routes to v2 handler", func() {
					req := httptest.NewRequest(http.MethodGet, "/v2/users", nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(http.StatusOK))
					Expect(w.Body.String()).To(Equal(`version: v2`))
				})

				It("does not include deprecation headers", func() {
					req := httptest.NewRequest(http.MethodGet, "/v2/users", nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(w.Header().Get("Deprecation")).To(BeEmpty())
				})

				It("routes to v3 handler", func() {
					req := httptest.NewRequest(http.MethodGet, "/v3/users", nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(http.StatusOK))
					Expect(w.Body.String()).To(Equal(`version: v3`))
				})
			})
		})

		Describe("Accept-header versioning", func() {
			It("selects version from Accept header", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("Accept", "application/vnd.api.v2+json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal(`version: v2`))
			})

			It("does not include deprecation for non-deprecated version", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("Accept", "application/vnd.api.v2+json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Header().Get("Deprecation")).To(BeEmpty())
			})

			It("selects deprecated version from Accept header", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("Accept", "application/vnd.api.v1+json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal(`version: v1`))
				Expect(w.Header().Get("Deprecation")).To(Equal("true"))
			})
		})

		Describe("Header-based versioning", func() {
			Context("with invalid version", func() {
				It("falls back to default version", func() {
					req := httptest.NewRequest(http.MethodGet, "/users", nil)
					req.Header.Set("X-Api-Version", "v99")
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					// Should use default version (v1)
					Expect(w.Code).To(Equal(http.StatusOK))
					Expect(w.Body.String()).To(Equal(`version: v1`))
				})

				It("records invalid version attempt", func() {
					req := httptest.NewRequest(http.MethodGet, "/users", nil)
					req.Header.Set("X-Api-Version", "v99")
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(invalidVersions).To(ContainElement("v99"))
				})
			})

			Context("with valid version", func() {
				It("routes to specified version", func() {
					req := httptest.NewRequest(http.MethodGet, "/users", nil)
					req.Header.Set("X-Api-Version", "v3")
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(http.StatusOK))
					Expect(w.Body.String()).To(Equal(`version: v3`))
				})

				It("routes to deprecated version with headers", func() {
					req := httptest.NewRequest(http.MethodGet, "/users", nil)
					req.Header.Set("X-Api-Version", "v1")
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(http.StatusOK))
					Expect(w.Body.String()).To(Equal(`version: v1`))
					Expect(w.Header().Get("Deprecation")).To(Equal("true"))
				})
			})
		})

		Describe("Query parameter versioning", func() {
			It("selects version from query parameter", func() {
				req := httptest.NewRequest(http.MethodGet, "/users?v=v2", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal(`version: v2`))
			})

			It("handles invalid query version", func() {
				req := httptest.NewRequest(http.MethodGet, "/users?v=invalid", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				// Should fallback to default
				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).To(Equal(`version: v1`))
			})
		})

		Describe("Version detection priority", func() {
			It("prioritizes path over header", func() {
				req := httptest.NewRequest(http.MethodGet, "/v2/users", nil)
				req.Header.Set("X-Api-Version", "v1") // Should be ignored
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Body.String()).To(Equal(`version: v2`))
			})

			It("prioritizes header over query", func() {
				req := httptest.NewRequest(http.MethodGet, "/users?v=v1", nil)
				req.Header.Set("X-Api-Version", "v2") // Should take precedence
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Body.String()).To(Equal(`version: v2`))
			})

			It("prioritizes path over accept header", func() {
				req := httptest.NewRequest(http.MethodGet, "/v3/users", nil)
				req.Header.Set("Accept", "application/vnd.api.v1+json") // Should be ignored
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(w.Body.String()).To(Equal(`version: v3`))
			})
		})

		Describe("Version observer callbacks", func() {
			It("calls version detected callback for valid versions", func() {
				req := httptest.NewRequest(http.MethodGet, "/v2/users", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(detectedVersions).To(ContainElement("v2"))
			})

			It("calls invalid version callback for invalid versions", func() {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("X-Api-Version", "v99")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)

				Expect(invalidVersions).To(ContainElement("v99"))
			})
		})
	})
})
