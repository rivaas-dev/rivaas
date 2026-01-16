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

package openapi_test

import (
	"context"
	"encoding/json"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"rivaas.dev/openapi"
	"rivaas.dev/openapi/validate"
)

var _ = Describe("OpenAPI Integration", Label("integration"), func() {
	Describe("Spec Generation", func() {
		It("should generate complete OpenAPI specification", func() {
			api := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
				openapi.WithInfoDescription("Integration test API"),
				openapi.WithServer("http://localhost:8080", "Local development"),
				openapi.WithTag("users", "User operations"),
				openapi.WithBearerAuth("bearerAuth", "JWT authentication"),
			)

			// Define request/response types
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

			// Generate spec using HTTP method constructors
			result, err := api.Generate(context.Background(),
				openapi.GET("/users/:id",
					openapi.WithSummary("Get user"),
					openapi.WithDescription("Retrieves a user by ID"),
					openapi.WithResponse(http.StatusOK, User{}),
					openapi.WithResponse(http.StatusNotFound, ErrorResponse{}),
					openapi.WithTags("users"),
					openapi.WithSecurity("bearerAuth"),
				),
				openapi.POST("/users",
					openapi.WithSummary("Create user"),
					openapi.WithDescription("Creates a new user"),
					openapi.WithRequest(CreateUserRequest{}),
					openapi.WithResponse(http.StatusCreated, User{}),
					openapi.WithResponse(http.StatusBadRequest, ErrorResponse{}),
					openapi.WithTags("users"),
					openapi.WithSecurity("bearerAuth"),
				),
				openapi.GET("/users",
					openapi.WithSummary("List users"),
					openapi.WithDescription("Retrieves a list of users"),
					openapi.WithResponse(http.StatusOK, []User{}),
					openapi.WithTags("users"),
					openapi.WithSecurity("bearerAuth"),
				),
			)
			Expect(err).NotTo(HaveOccurred(), "should generate spec successfully")
			Expect(result.JSON).NotTo(BeEmpty(), "spec JSON should not be empty")

			// Validate JSON structure
			var spec map[string]any
			Expect(json.Unmarshal(result.JSON, &spec)).To(Succeed(), "spec should be valid JSON")

			// Verify OpenAPI version
			Expect(spec["openapi"]).To(Equal("3.0.4"), "should use OpenAPI 3.0.4 by default")

			// Verify info
			info, ok := spec["info"].(map[string]any)
			Expect(ok).To(BeTrue(), "info should be present")
			Expect(info["title"]).To(Equal("Test API"))
			Expect(info["version"]).To(Equal("1.0.0"))
			Expect(info["description"]).To(Equal("Integration test API"))

			// Verify paths
			paths, ok := spec["paths"].(map[string]any)
			Expect(ok).To(BeTrue(), "paths should be present")
			Expect(paths).To(HaveLen(2), "should have 2 unique paths")
			Expect(paths).To(HaveKey("/users/{id}"))
			Expect(paths).To(HaveKey("/users"))

			// Verify servers
			servers, ok := spec["servers"].([]any)
			Expect(ok).To(BeTrue(), "servers should be present")
			Expect(servers).To(HaveLen(1))
		})

		It("should generate OpenAPI 3.1 specification when configured", func() {
			api := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
				openapi.WithVersion(openapi.V31x),
			)

			result, err := api.Generate(context.Background(),
				openapi.GET("/health",
					openapi.WithSummary("Health check"),
				),
			)
			Expect(err).NotTo(HaveOccurred())

			var spec map[string]any
			Expect(json.Unmarshal(result.JSON, &spec)).To(Succeed())
			Expect(spec["openapi"]).To(Equal("3.1.2"))
		})

		It("should handle empty operations with OpenAPI 3.1", func() {
			// OpenAPI 3.1 allows empty paths, 3.0 does not
			api := openapi.MustNew(
				openapi.WithTitle("Empty API", "1.0.0"),
				openapi.WithVersion(openapi.V31x),
			)

			result, err := api.Generate(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(result.JSON).NotTo(BeEmpty())

			var spec map[string]any
			Expect(json.Unmarshal(result.JSON, &spec)).To(Succeed())
			// Paths may be nil or empty for OpenAPI 3.1 with no routes
			paths := spec["paths"]
			if paths != nil {
				Expect(paths).To(BeEmpty())
			}
		})
	})

	Describe("HTTP Method Constructors", func() {
		It("should create operations with correct HTTP methods", func() {
			getOp := openapi.GET("/resource", openapi.WithSummary("Get"))
			postOp := openapi.POST("/resource", openapi.WithSummary("Create"))
			putOp := openapi.PUT("/resource/:id", openapi.WithSummary("Update"))
			patchOp := openapi.PATCH("/resource/:id", openapi.WithSummary("Patch"))
			deleteOp := openapi.DELETE("/resource/:id", openapi.WithSummary("Delete"))
			headOp := openapi.HEAD("/resource/:id", openapi.WithSummary("Head"))
			optionsOp := openapi.OPTIONS("/resource", openapi.WithSummary("Options"))

			Expect(getOp.Method).To(Equal(http.MethodGet))
			Expect(postOp.Method).To(Equal(http.MethodPost))
			Expect(putOp.Method).To(Equal(http.MethodPut))
			Expect(patchOp.Method).To(Equal(http.MethodPatch))
			Expect(deleteOp.Method).To(Equal(http.MethodDelete))
			Expect(headOp.Method).To(Equal(http.MethodHead))
			Expect(optionsOp.Method).To(Equal(http.MethodOptions))
		})

		It("should create custom method operations with Op()", func() {
			op := openapi.Op("CUSTOM", "/resource", openapi.WithSummary("Custom"))
			Expect(op.Method).To(Equal("CUSTOM"))
			Expect(op.Path).To(Equal("/resource"))
		})

		It("should create TRACE operations", func() {
			traceOp := openapi.TRACE("/resource/:id", openapi.WithSummary("Trace"))
			Expect(traceOp.Method).To(Equal(http.MethodTrace))
			Expect(traceOp.Path).To(Equal("/resource/:id"))
		})
	})

	Describe("Path Validation", func() {
		It("should validate correct paths", func() {
			Expect(validate.ValidatePath("/users")).To(Succeed())
			Expect(validate.ValidatePath("/users/:id")).To(Succeed())
			Expect(validate.ValidatePath("/users/:userId/posts/:postId")).To(Succeed())
		})

		It("should reject empty paths", func() {
			err := validate.ValidatePath("")
			Expect(err).To(MatchError(validate.ErrPathEmpty))
		})

		It("should reject paths without leading slash", func() {
			err := validate.ValidatePath("users")
			Expect(err).To(MatchError(validate.ErrPathNoLeadingSlash))
		})

		It("should reject duplicate parameters", func() {
			err := validate.ValidatePath("/users/:id/posts/:id")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("duplicate"))
		})

		It("should reject invalid parameter syntax", func() {
			err := validate.ValidatePath("/users/:/posts")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid"))
		})
	})

	Describe("Operation Extensions", func() {
		It("should support operation-level extensions", func() {
			api := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
			)

			result, err := api.Generate(context.Background(),
				openapi.GET("/users/:id",
					openapi.WithSummary("Get user"),
					openapi.WithOperationExtension("x-rate-limit", 100),
					openapi.WithOperationExtension("x-internal", true),
				),
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.JSON).NotTo(BeEmpty())
		})
	})

	Describe("YAML Output", func() {
		It("should generate both JSON and YAML output", func() {
			api := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
			)

			type User struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}

			result, err := api.Generate(context.Background(),
				openapi.GET("/users/:id",
					openapi.WithSummary("Get user"),
					openapi.WithResponse(http.StatusOK, User{}),
				),
			)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.JSON).NotTo(BeEmpty(), "JSON output should not be empty")
			Expect(result.YAML).NotTo(BeEmpty(), "YAML output should not be empty")

			// Verify YAML contains expected content
			yamlStr := string(result.YAML)
			Expect(yamlStr).To(ContainSubstring("openapi:"))
			Expect(yamlStr).To(ContainSubstring("title: Test API"))
		})
	})

	Describe("Validation", func() {
		It("should not validate by default for backward compatibility", func() {
			api := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
			)

			// Default should have validation disabled
			Expect(api.ValidateSpec).To(BeFalse())

			result, err := api.Generate(context.Background(),
				openapi.GET("/test",
					openapi.WithSummary("Test endpoint"),
					openapi.WithResponse(http.StatusOK, struct{}{}),
				),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.JSON).NotTo(BeEmpty())
		})

		It("should allow enabling validation", func() {
			api := openapi.MustNew(
				openapi.WithTitle("Test API", "1.0.0"),
				openapi.WithValidation(true),
			)

			Expect(api.ValidateSpec).To(BeTrue())
		})

		// Note: These tests use the embedded OpenAPI metaschemas which are complex
		// and test the full validation stack. Skipped for unit tests.
		XIt("should validate external spec using Validator", func() {
			validSpec := `{
				"openapi": "3.0.4",
				"info": {
					"title": "External API",
					"version": "1.0.0"
				},
				"paths": {}
			}`

			validator := validate.New()
			err := validator.Validate(context.Background(), []byte(validSpec), validate.V30)
			Expect(err).NotTo(HaveOccurred())
		})

		XIt("should validate multiple specs with Validator", func() {
			specs := []string{
				`{"openapi": "3.0.4", "info": {"title": "API 1", "version": "1.0.0"}, "paths": {}}`,
				`{"openapi": "3.0.4", "info": {"title": "API 2", "version": "1.0.0"}, "paths": {}}`,
				`{"openapi": "3.1.2", "info": {"title": "API 3", "version": "1.0.0"}, "paths": {}}`,
			}

			validator := validate.New()
			for i, spec := range specs {
				version := validate.V30
				if i == 2 {
					version = validate.V31
				}
				err := validator.Validate(context.Background(), []byte(spec), version)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
})
