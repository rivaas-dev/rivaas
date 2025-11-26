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

package openapi_test

import (
	"encoding/json"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"rivaas.dev/openapi"
	"rivaas.dev/openapi/example"
)

var _ = Describe("OpenAPI Integration", Label("integration"), func() {
	Describe("Spec Generation", func() {
		It("should generate complete OpenAPI specification", func() {
			cfg := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
				openapi.WithDescription("Integration test API"),
				openapi.WithServer("http://localhost:8080", "Local development"),
				openapi.WithTag("users", "User operations"),
				openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
			)

			manager := openapi.NewManager(cfg)

			// Register multiple routes
			type GetUserRequest struct {
				ID int `params:"id" doc:"User ID"`
			}

			type User struct {
				ID    int    `json:"id"`
				Name  string `json:"name"`
				Email string `json:"email"`
			}

			type CreateUserRequest struct {
				Name  string `json:"name" validate:"required"`
				Email string `json:"email" validate:"required,email"`
			}

			type ErrorResponse struct {
				Error string `json:"error"`
			}

			manager.Register("GET", "/users/:id").
				Doc("Get user", "Retrieves a user by ID").
				Request(GetUserRequest{}).
				Response(200, User{}).
				Response(404, ErrorResponse{}).
				Tags("users").
				Security("bearerAuth")

			manager.Register("POST", "/users").
				Doc("Create user", "Creates a new user").
				Request(CreateUserRequest{}).
				Response(201, User{}).
				Response(400, ErrorResponse{}).
				Tags("users").
				Security("bearerAuth")

			manager.Register("GET", "/users").
				Doc("List users", "Retrieves a list of users").
				Response(200, []User{}).
				Tags("users").
				Security("bearerAuth")

			// Generate spec
			specJSON, etag, err := manager.GenerateSpec()
			Expect(err).NotTo(HaveOccurred(), "should generate spec successfully")
			Expect(specJSON).NotTo(BeEmpty(), "spec JSON should not be empty")
			Expect(etag).NotTo(BeEmpty(), "ETag should not be empty")

			// Validate JSON structure
			var spec map[string]any
			Expect(json.Unmarshal(specJSON, &spec)).To(Succeed(), "spec should be valid JSON")

			// Verify OpenAPI version
			Expect(spec["openapi"]).To(Equal("3.0.4"), "should use OpenAPI 3.0.4 by default")

			// Verify info
			info, ok := spec["info"].(map[string]any)
			Expect(ok).To(BeTrue(), "info should be present")
			Expect(info["title"]).To(Equal("Test API"))
			Expect(info["version"]).To(Equal("1.0.0"))
			Expect(info["description"]).To(Equal("Integration test API"))

			// Verify servers
			servers, ok := spec["servers"].([]any)
			Expect(ok).To(BeTrue(), "servers should be present")
			Expect(servers).To(HaveLen(1))
			server, ok := servers[0].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(server["url"]).To(Equal("http://localhost:8080"))

			// Verify paths
			paths, ok := spec["paths"].(map[string]any)
			Expect(ok).To(BeTrue(), "paths should be present")
			Expect(paths).To(HaveKey("/users/{id}"), "should contain GET /users/:id")
			Expect(paths).To(HaveKey("/users"), "should contain POST /users and GET /users")

			// Verify security schemes
			components, ok := spec["components"].(map[string]any)
			Expect(ok).To(BeTrue(), "components should be present")
			securitySchemes, ok := components["securitySchemes"].(map[string]any)
			Expect(ok).To(BeTrue(), "securitySchemes should be present")
			Expect(securitySchemes).To(HaveKey("bearerAuth"), "should contain bearerAuth scheme")

			// Verify tags
			tags, ok := spec["tags"].([]any)
			Expect(ok).To(BeTrue(), "tags should be present")
			Expect(tags).To(HaveLen(1))
			tag, ok := tags[0].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(tag["name"]).To(Equal("users"))
		})
	})

	Describe("Spec Caching", func() {
		It("should cache generated specs until routes change", func() {
			cfg := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
			)

			manager := openapi.NewManager(cfg)
			manager.Register("GET", "/test").
				Doc("Test endpoint", "A test endpoint").
				Response(200, map[string]string{"message": "test"})

			// Generate spec first time
			specJSON1, etag1, err := manager.GenerateSpec()
			Expect(err).NotTo(HaveOccurred())

			// Generate spec second time - should use cache
			specJSON2, etag2, err := manager.GenerateSpec()
			Expect(err).NotTo(HaveOccurred())

			// ETags should be the same (cached)
			Expect(etag2).To(Equal(etag1), "ETags should match when using cache")
			Expect(string(specJSON2)).To(Equal(string(specJSON1)), "specs should be identical when cached")

			// Register new route - should invalidate cache
			manager.Register("POST", "/test").
				Doc("Create test", "Creates a test").
				Response(201, map[string]string{"id": "string"})

			// Generate spec third time - should regenerate
			specJSON3, etag3, err := manager.GenerateSpec()
			Expect(err).NotTo(HaveOccurred())

			// ETag should be different (cache invalidated)
			Expect(etag3).NotTo(Equal(etag2), "ETag should change after cache invalidation")

			// Spec should be different (contains new route)
			var spec1, spec3 map[string]any
			Expect(json.Unmarshal(specJSON1, &spec1)).To(Succeed())
			Expect(json.Unmarshal(specJSON3, &spec3)).To(Succeed())

			paths1 := spec1["paths"].(map[string]any)
			paths3 := spec3["paths"].(map[string]any)

			Expect(paths1).To(HaveLen(1), "first spec should have 1 path")
			Expect(paths3).To(HaveLen(1), "third spec should have 1 path (same path, different methods)")

			// Verify POST method was added
			testPath, ok := paths3["/test"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(testPath).To(HaveKey("get"), "should contain GET method")
			Expect(testPath).To(HaveKey("post"), "should contain POST method")
		})
	})

	Describe("Multiple Routes", func() {
		It("should handle multiple routes across different tags", func() {
			cfg := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
				openapi.WithTag("users", "User operations"),
				openapi.WithTag("orders", "Order operations"),
			)

			manager := openapi.NewManager(cfg)

			type User struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}

			type Order struct {
				ID     int `json:"id"`
				UserID int `json:"user_id"`
			}

			// Register multiple routes across different tags
			manager.Register("GET", "/users/:id").
				Doc("Get user", "Retrieves a user").
				Response(200, User{}).
				Tags("users")

			manager.Register("GET", "/users").
				Doc("List users", "Lists all users").
				Response(200, []User{}).
				Tags("users")

			manager.Register("GET", "/orders/:id").
				Doc("Get order", "Retrieves an order").
				Response(200, Order{}).
				Tags("orders")

			manager.Register("POST", "/orders").
				Doc("Create order", "Creates a new order").
				Request(Order{}).
				Response(201, Order{}).
				Tags("orders")

			// Generate spec
			specJSON, _, err := manager.GenerateSpec()
			Expect(err).NotTo(HaveOccurred())

			var spec map[string]any
			Expect(json.Unmarshal(specJSON, &spec)).To(Succeed())

			// Verify all routes are present
			paths, ok := spec["paths"].(map[string]any)
			Expect(ok).To(BeTrue())

			Expect(paths).To(HaveKey("/users/{id}"), "should contain GET /users/:id")
			Expect(paths).To(HaveKey("/users"), "should contain GET /users")
			Expect(paths).To(HaveKey("/orders/{id}"), "should contain GET /orders/:id")
			Expect(paths).To(HaveKey("/orders"), "should contain POST /orders")

			// Verify tags are present
			tags, ok := spec["tags"].([]any)
			Expect(ok).To(BeTrue())
			Expect(tags).To(HaveLen(2), "should have 2 tags")

			tagNames := make(map[string]bool)
			for _, tag := range tags {
				tagMap, ok := tag.(map[string]any)
				Expect(ok).To(BeTrue())
				name, ok := tagMap["name"].(string)
				Expect(ok).To(BeTrue())
				tagNames[name] = true
			}

			Expect(tagNames).To(HaveKey("users"), "should have users tag")
			Expect(tagNames).To(HaveKey("orders"), "should have orders tag")
		})
	})

	Describe("Swagger UI Configuration", func() {
		It("should configure Swagger UI correctly", func() {
			cfg := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
				openapi.WithSwaggerUI(true, "/docs"),
			)

			manager := openapi.NewManager(cfg)
			manager.Register("GET", "/test").
				Doc("Test endpoint", "A test endpoint").
				Response(200, map[string]string{"message": "test"})

			// Generate spec
			specJSON, _, err := manager.GenerateSpec()
			Expect(err).NotTo(HaveOccurred())

			// Verify Swagger UI configuration
			Expect(cfg.ServeUI).To(BeTrue(), "UI should be enabled")
			Expect(cfg.UIPath).To(Equal("/docs"), "UI path should be set")
			Expect(cfg.SpecPath).To(Equal("/openapi.json"), "spec path should be default")

			// Verify spec is valid JSON
			var spec map[string]any
			Expect(json.Unmarshal(specJSON, &spec)).To(Succeed(), "spec should be valid JSON")

			// In a real integration test, we would test HTTP serving:
			// - Create HTTP server
			// - Register Swagger UI handler
			// - Make requests to /docs and /openapi.json
			// - Verify responses
			// For now, we verify the configuration is correct
		})
	})

	Describe("OpenAPI 3.1 Features", func() {
		It("should generate OpenAPI 3.1.2 specification with 3.1-only features", func() {
			cfg := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
				openapi.WithVersion(openapi.Version31),
				openapi.WithSummary("API summary"), // 3.1-only feature
			)

			manager := openapi.NewManager(cfg)
			manager.Register("GET", "/test").
				Doc("Test endpoint", "A test endpoint").
				Response(200, map[string]string{"message": "test"})

			// Generate spec
			specJSON, _, err := manager.GenerateSpec()
			Expect(err).NotTo(HaveOccurred())

			var spec map[string]any
			Expect(json.Unmarshal(specJSON, &spec)).To(Succeed())

			// Verify OpenAPI version
			Expect(spec["openapi"]).To(Equal("3.1.2"), "should use OpenAPI 3.1.2")

			// Verify 3.1-only features
			info, ok := spec["info"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(info["summary"]).To(Equal("API summary"), "should include summary (3.1 feature)")
		})
	})

	Describe("Concurrent Spec Generation", func() {
		It("should safely generate specs concurrently", func() {
			cfg := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
			)

			manager := openapi.NewManager(cfg)

			// Register routes
			for i := 0; i < 10; i++ {
				manager.Register("GET", "/test"+string(rune('0'+i))).
					Doc("Test endpoint", "A test endpoint").
					Response(200, map[string]string{"id": "string"})
			}

			// Concurrently generate specs
			const numGoroutines = 10
			results := make([]struct {
				specJSON []byte
				etag     string
				err      error
			}, numGoroutines)

			var wg sync.WaitGroup
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					specJSON, etag, err := manager.GenerateSpec()
					results[idx] = struct {
						specJSON []byte
						etag     string
						err      error
					}{specJSON, etag, err}
				}(i)
			}
			wg.Wait()

			// All should succeed
			for i, result := range results {
				Expect(result.err).NotTo(HaveOccurred(), "goroutine %d should succeed", i)
				Expect(result.specJSON).NotTo(BeEmpty(), "goroutine %d should generate spec", i)
				Expect(result.etag).NotTo(BeEmpty(), "goroutine %d should generate ETag", i)
			}

			// All ETags should be the same (same spec)
			firstETag := results[0].etag
			for i, result := range results {
				Expect(result.etag).To(Equal(firstETag), "goroutine %d should have same ETag", i)
				Expect(string(result.specJSON)).To(Equal(string(results[0].specJSON)), "goroutine %d should have same spec", i)
			}
		})
	})

	Describe("Example API Integration", func() {
		var manager *openapi.Manager
		var cfg *openapi.Config

		BeforeEach(func() {
			cfg = openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
				openapi.WithVersion(openapi.Version31),
			)
			manager = openapi.NewManager(cfg)
		})

		Context("Response Examples", func() {
			Context("with named examples", func() {
				It("should generate examples map in spec", func() {
					type User struct {
						ID   int    `json:"id"`
						Name string `json:"name"`
					}

					route := manager.Register("GET", "/users/:id")
					route.Response(200, User{},
						example.New("success", User{ID: 123, Name: "John"},
							example.WithSummary("Successful lookup")),
					)

					specJSON, _, err := manager.GenerateSpec()
					Expect(err).NotTo(HaveOccurred())

					var spec map[string]any
					Expect(json.Unmarshal(specJSON, &spec)).To(Succeed())

					paths := spec["paths"].(map[string]any)
					pathItem := paths["/users/{id}"].(map[string]any)
					getOp := pathItem["get"].(map[string]any)
					responses := getOp["responses"].(map[string]any)
					response200 := responses["200"].(map[string]any)
					content := response200["content"].(map[string]any)
					mt := content["application/json"].(map[string]any)

					Expect(mt).To(HaveKey("examples"))
					examples := mt["examples"].(map[string]any)
					Expect(examples).To(HaveKey("success"))
					successEx := examples["success"].(map[string]any)
					Expect(successEx["summary"]).To(Equal("Successful lookup"))
				})
			})

			Context("with single example (no named)", func() {
				It("should generate example field in spec", func() {
					type User struct {
						ID   int    `json:"id"`
						Name string `json:"name"`
					}

					route := manager.Register("GET", "/users/:id")
					route.Response(200, User{ID: 123, Name: "John"})

					specJSON, _, err := manager.GenerateSpec()
					Expect(err).NotTo(HaveOccurred())

					var spec map[string]any
					Expect(json.Unmarshal(specJSON, &spec)).To(Succeed())

					paths := spec["paths"].(map[string]any)
					pathItem := paths["/users/{id}"].(map[string]any)
					getOp := pathItem["get"].(map[string]any)
					responses := getOp["responses"].(map[string]any)
					response200 := responses["200"].(map[string]any)
					content := response200["content"].(map[string]any)
					mt := content["application/json"].(map[string]any)

					Expect(mt).To(HaveKey("example"))
					Expect(mt["example"]).NotTo(BeNil())
					Expect(mt).NotTo(HaveKey("examples"))
				})
			})
		})

		Describe("Request Examples", func() {
			Context("with named examples", func() {
				It("should generate examples map in request body", func() {
					type CreateUser struct {
						Name  string `json:"name"`
						Email string `json:"email"`
					}

					route := manager.Register("POST", "/users")
					route.Request(CreateUser{},
						example.New("minimal", CreateUser{Name: "John"}),
						example.New("complete", CreateUser{Name: "John", Email: "john@example.com"}),
					).Response(201, CreateUser{})

					specJSON, _, err := manager.GenerateSpec()
					Expect(err).NotTo(HaveOccurred())

					var spec map[string]any
					Expect(json.Unmarshal(specJSON, &spec)).To(Succeed())

					paths := spec["paths"].(map[string]any)
					pathItem := paths["/users"].(map[string]any)
					postOp := pathItem["post"].(map[string]any)
					requestBody := postOp["requestBody"].(map[string]any)
					content := requestBody["content"].(map[string]any)
					mt := content["application/json"].(map[string]any)

					Expect(mt).To(HaveKey("examples"))
					examples := mt["examples"].(map[string]any)
					Expect(examples).To(HaveLen(2))
					Expect(examples).To(HaveKey("minimal"))
					Expect(examples).To(HaveKey("complete"))
				})
			})
		})
	})
})
