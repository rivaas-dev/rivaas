package build

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/openapi/internal/schema"
	"rivaas.dev/openapi/model"
)

type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// newTestBuilder creates a new Builder for testing.
func newTestBuilder(tb testing.TB) *Builder {
	tb.Helper()
	return NewBuilder(model.Info{
		Title:   "Test API",
		Version: "1.0.0",
	})
}

func TestBuilder_Build(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{
				Method: string(http.MethodGet),
				Path:   "/users/:id",
			},
			Doc: &RouteDoc{
				Summary: "Get user",
				ResponseTypes: map[int]reflect.Type{
					200: reflect.TypeOf(User{}),
				},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)
	require.NotNil(t, spec)

	assert.Equal(t, "Test API", spec.Info.Title)
	assert.Equal(t, "1.0.0", spec.Info.Version)
	assert.Contains(t, spec.Paths, "/users/{id}")

	pathItem := spec.Paths["/users/{id}"]
	require.NotNil(t, pathItem.Get)
	assert.Equal(t, "Get user", pathItem.Get.Summary)
	assert.Equal(t, "getUserById", pathItem.Get.OperationID)
	assert.Contains(t, pathItem.Get.Responses, "200")
}

func TestBuilder_OperationIDGeneration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method   string
		path     string
		expected string
	}{
		{string(http.MethodGet), "/users", "getUsers"},
		{string(http.MethodGet), "/users/:id", "getUserById"},
		{string(http.MethodPost), "/users", "createUser"},
		{string(http.MethodPut), "/users/:id", "replaceUserById"},
		{string(http.MethodPatch), "/users/:id", "updateUserById"},
		{string(http.MethodDelete), "/users/:id", "deleteUserById"},
		{string(http.MethodGet), "/users/:id/orders", "getUserOrdersById"},
		{string(http.MethodGet), "/users/:id/orders/:orderId", "getUserOrderByOrderId"},
		{string(http.MethodPost), "/users/:userId/posts", "createUserPostByUserId"},
		{string(http.MethodGet), "/", "getRoot"},
		{string(http.MethodGet), "/health", "getHealth"},
	}

	builder := newTestBuilder(t)

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			t.Parallel()
			routes := []EnrichedRoute{
				{
					RouteInfo: RouteInfo{Method: tt.method, Path: tt.path},
					Doc: &RouteDoc{
						ResponseTypes: map[int]reflect.Type{200: nil},
					},
				},
			}

			spec, err := builder.Build(routes)
			require.NoError(t, err)

			// Find the operation
			var op *model.Operation
			for _, pathItem := range spec.Paths {
				switch tt.method {
				case http.MethodGet:
					op = pathItem.Get
				case http.MethodPost:
					op = pathItem.Post
				case http.MethodPut:
					op = pathItem.Put
				case http.MethodPatch:
					op = pathItem.Patch
				case http.MethodDelete:
					op = pathItem.Delete
				}
				if op != nil {
					break
				}
			}

			require.NotNil(t, op, "operation not found for %s %s", tt.method, tt.path)
			assert.Equal(t, tt.expected, op.OperationID)
		})
	}
}

func TestBuilder_CustomOperationID(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: string(http.MethodGet), Path: "/users"},
			Doc: &RouteDoc{
				OperationID:   "customGetUsers",
				ResponseTypes: map[int]reflect.Type{200: nil},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/users"]
	require.NotNil(t, pathItem.Get)
	assert.Equal(t, "customGetUsers", pathItem.Get.OperationID)
}

func TestBuilder_DuplicateOperationID(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/users"},
			Doc: &RouteDoc{
				OperationID:   "getUsers",
				ResponseTypes: map[int]reflect.Type{http.StatusOK: nil},
			},
		},
		{
			RouteInfo: RouteInfo{Method: http.MethodPost, Path: "/users"},
			Doc: &RouteDoc{
				OperationID:   "getUsers", // Duplicate!
				ResponseTypes: map[int]reflect.Type{http.StatusOK: nil},
			},
		},
	}

	_, err := builder.Build(routes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate operation ID")
	assert.Contains(t, err.Error(), "getUsers")
}

func TestBuilder_MultipleMethodsOnPath(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/users/:id"},
			Doc: &RouteDoc{
				Summary:       "Get user",
				ResponseTypes: map[int]reflect.Type{http.StatusOK: nil},
			},
		},
		{
			RouteInfo: RouteInfo{Method: http.MethodPut, Path: "/users/:id"},
			Doc: &RouteDoc{
				Summary:       "Update user",
				ResponseTypes: map[int]reflect.Type{http.StatusOK: nil},
			},
		},
		{
			RouteInfo: RouteInfo{Method: http.MethodDelete, Path: "/users/:id"},
			Doc: &RouteDoc{
				Summary:       "Delete user",
				ResponseTypes: map[int]reflect.Type{http.StatusNoContent: nil},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/users/{id}"]
	require.NotNil(t, pathItem.Get)
	require.NotNil(t, pathItem.Put)
	require.NotNil(t, pathItem.Delete)

	assert.Equal(t, "Get user", pathItem.Get.Summary)
	assert.Equal(t, "Update user", pathItem.Put.Summary)
	assert.Equal(t, "Delete user", pathItem.Delete.Summary)
}

func TestBuilder_RequestBody(t *testing.T) {
	t.Parallel()

	type CreateUserRequest struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodPost, Path: "/users"},
			Doc: &RouteDoc{
				RequestType:     reflect.TypeOf(CreateUserRequest{}),
				RequestMetadata: schema.IntrospectRequest(reflect.TypeOf(CreateUserRequest{})),
				ResponseTypes:   map[int]reflect.Type{http.StatusCreated: nil},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/users"]
	require.NotNil(t, pathItem.Post)
	require.NotNil(t, pathItem.Post.RequestBody)
	assert.True(t, pathItem.Post.RequestBody.Required)
	assert.Contains(t, pathItem.Post.RequestBody.Content, "application/json")
}

func TestBuilder_Parameters(t *testing.T) {
	t.Parallel()

	type GetUserRequest struct {
		ID     int    `path:"id"`
		Expand string `query:"expand"`
	}

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/users/:id"},
			Doc: &RouteDoc{
				RequestType:   reflect.TypeOf(GetUserRequest{}),
				ResponseTypes: map[int]reflect.Type{http.StatusOK: nil},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/users/{id}"]
	require.NotNil(t, pathItem.Get)

	// Should have both path and query parameters
	assert.GreaterOrEqual(t, len(pathItem.Get.Parameters), 1)

	// Find path parameter
	var pathParam *model.Parameter
	for i := range pathItem.Get.Parameters {
		if pathItem.Get.Parameters[i].Name == "id" && pathItem.Get.Parameters[i].In == "path" {
			pathParam = &pathItem.Get.Parameters[i]
			break
		}
	}
	require.NotNil(t, pathParam)
	assert.True(t, pathParam.Required)
}

func TestBuilder_ComponentSchemas(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/users/:id"},
			Doc: &RouteDoc{
				ResponseTypes: map[int]reflect.Type{
					http.StatusOK: reflect.TypeOf(User{}),
				},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	require.NotNil(t, spec.Components)
	assert.Contains(t, spec.Components.Schemas, "build.User")

	userSchema := spec.Components.Schemas["build.User"]
	require.NotNil(t, userSchema)
	assert.Equal(t, model.KindObject, userSchema.Kind)
	assert.Contains(t, userSchema.Properties, "id")
	assert.Contains(t, userSchema.Properties, "name")
}

func TestBuilder_Servers(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)
	builder.AddServer("https://api.example.com", "Production")
	builder.AddServer("https://staging.example.com", "Staging")

	routes := []EnrichedRoute{}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	assert.Len(t, spec.Servers, 2)
	assert.Equal(t, "https://api.example.com", spec.Servers[0].URL)
	assert.Equal(t, "Production", spec.Servers[0].Description)
	assert.Equal(t, "https://staging.example.com", spec.Servers[1].URL)
	assert.Equal(t, "Staging", spec.Servers[1].Description)
}

func TestBuilder_Tags(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)
	builder.AddTag("users", "User management operations")
	builder.AddTag("orders", "Order operations")

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/users"},
			Doc: &RouteDoc{
				Tags:          []string{"users"},
				ResponseTypes: map[int]reflect.Type{http.StatusOK: nil},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	assert.Len(t, spec.Tags, 2)
	// Tags are sorted alphabetically
	assert.Equal(t, "orders", spec.Tags[0].Name)
	assert.Equal(t, "Order operations", spec.Tags[0].Description)
	assert.Equal(t, "users", spec.Tags[1].Name)
	assert.Equal(t, "User management operations", spec.Tags[1].Description)
}

func TestBuilder_NoDoc(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/users/:id"},
			Doc:       nil, // No documentation
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/users/{id}"]
	require.NotNil(t, pathItem.Get)
	// Should have default 200 response
	assert.Contains(t, pathItem.Get.Responses, "200")
	assert.Equal(t, "OK", pathItem.Get.Responses["200"].Description)
}

func TestBuilder_AllHTTPMethods(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/test"}, Doc: &RouteDoc{ResponseTypes: map[int]reflect.Type{http.StatusOK: nil}}},
		{RouteInfo: RouteInfo{Method: http.MethodPost, Path: "/test"}, Doc: &RouteDoc{ResponseTypes: map[int]reflect.Type{http.StatusCreated: nil}}},
		{RouteInfo: RouteInfo{Method: http.MethodPut, Path: "/test"}, Doc: &RouteDoc{ResponseTypes: map[int]reflect.Type{http.StatusOK: nil}}},
		{RouteInfo: RouteInfo{Method: http.MethodPatch, Path: "/test"}, Doc: &RouteDoc{ResponseTypes: map[int]reflect.Type{http.StatusOK: nil}}},
		{RouteInfo: RouteInfo{Method: http.MethodDelete, Path: "/test"}, Doc: &RouteDoc{ResponseTypes: map[int]reflect.Type{http.StatusNoContent: nil}}},
		{RouteInfo: RouteInfo{Method: http.MethodHead, Path: "/test"}, Doc: &RouteDoc{ResponseTypes: map[int]reflect.Type{http.StatusOK: nil}}},
		{RouteInfo: RouteInfo{Method: http.MethodOptions, Path: "/test"}, Doc: &RouteDoc{ResponseTypes: map[int]reflect.Type{http.StatusOK: nil}}},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/test"]
	require.NotNil(t, pathItem)
	assert.NotNil(t, pathItem.Get)
	assert.NotNil(t, pathItem.Post)
	assert.NotNil(t, pathItem.Put)
	assert.NotNil(t, pathItem.Patch)
	assert.NotNil(t, pathItem.Delete)
	assert.NotNil(t, pathItem.Head)
	assert.NotNil(t, pathItem.Options)
}

func TestBuilder_MultipleResponseCodes(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodPost, Path: "/users"},
			Doc: &RouteDoc{
				ResponseTypes: map[int]reflect.Type{
					http.StatusCreated:             reflect.TypeOf(User{}),
					http.StatusBadRequest:          nil,
					http.StatusUnauthorized:        nil,
					http.StatusInternalServerError: nil,
				},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/users"]
	require.NotNil(t, pathItem.Post)

	responses := pathItem.Post.Responses
	assert.Contains(t, responses, "201")
	assert.Contains(t, responses, "400")
	assert.Contains(t, responses, "401")
	assert.Contains(t, responses, "500")

	// 201 should have content (User type)
	assert.NotNil(t, responses["201"].Content)
	// 400, 401, 500 should not have content
	assert.Nil(t, responses["400"].Content)
	assert.Nil(t, responses["401"].Content)
	assert.Nil(t, responses["500"].Content)
}

func TestBuilder_DeprecatedOperation(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/old"},
			Doc: &RouteDoc{
				Deprecated:    true,
				ResponseTypes: map[int]reflect.Type{http.StatusOK: nil},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/old"]
	require.NotNil(t, pathItem.Get)
	assert.True(t, pathItem.Get.Deprecated)
}

func TestBuilder_SecurityRequirements(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)
	builder.AddSecurityScheme("bearerAuth", &model.SecurityScheme{
		Type:         "http",
		Scheme:       "bearer",
		BearerFormat: "JWT",
	})

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/secure"},
			Doc: &RouteDoc{
				Security: []SecurityReq{
					{Scheme: "bearerAuth", Scopes: []string{"read", "write"}},
				},
				ResponseTypes: map[int]reflect.Type{http.StatusOK: nil},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/secure"]
	require.NotNil(t, pathItem.Get)
	require.Len(t, pathItem.Get.Security, 1)
	assert.Contains(t, pathItem.Get.Security[0], "bearerAuth")
	assert.Equal(t, []string{"read", "write"}, pathItem.Get.Security[0]["bearerAuth"])
}

func TestBuilder_GlobalSecurity(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)
	builder.AddSecurityScheme("bearerAuth", &model.SecurityScheme{
		Type:   "http",
		Scheme: "bearer",
	})
	builder.SetGlobalSecurity([]model.SecurityRequirement{
		{"bearerAuth": []string{}},
	})

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/users"},
			Doc: &RouteDoc{
				ResponseTypes: map[int]reflect.Type{http.StatusOK: nil},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	assert.Len(t, spec.Security, 1)
	assert.Contains(t, spec.Security[0], "bearerAuth")
}

func TestBuilder_EmptyRoutes(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	spec, err := builder.Build([]EnrichedRoute{})
	require.NoError(t, err)
	require.NotNil(t, spec)
	assert.Empty(t, spec.Paths)
}

func TestBuilder_NilDoc(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/test"},
			Doc:       nil, // Test nil doc handling
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)
	require.NotNil(t, spec)
	assert.NotEmpty(t, spec.Paths)

	pathItem := spec.Paths["/test"]
	require.NotNil(t, pathItem.Get)
	// Should have default 200 response
	assert.Contains(t, pathItem.Get.Responses, "200")
	assert.Equal(t, "OK", pathItem.Get.Responses["200"].Description)
	// Should have path parameters extracted from route (may be empty)
	assert.NotNil(t, pathItem.Get.Parameters)
}

func TestBuilder_ComplexPath(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/users/:userId/posts/:postId/comments/:commentId"},
			Doc: &RouteDoc{
				ResponseTypes: map[int]reflect.Type{http.StatusOK: nil},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/users/{userId}/posts/{postId}/comments/{commentId}"]
	require.NotNil(t, pathItem.Get)
	assert.Equal(t, "getUserPostCommentByCommentId", pathItem.Get.OperationID)

	// Should have all path parameters
	paramNames := make(map[string]bool)
	for _, p := range pathItem.Get.Parameters {
		if p.In == "path" {
			paramNames[p.Name] = true
		}
	}
	assert.True(t, paramNames["userId"])
	assert.True(t, paramNames["postId"])
	assert.True(t, paramNames["commentId"])
}

func TestBuilder_ResponseExamples(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/users/:id"},
			Doc: &RouteDoc{
				ResponseTypes: map[int]reflect.Type{
					http.StatusOK: reflect.TypeOf(User{}),
				},
				ResponseExample: map[int]any{
					http.StatusOK: User{ID: 1, Name: "John"},
				},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/users/{id}"]
	require.NotNil(t, pathItem.Get)
	response := pathItem.Get.Responses["200"]
	require.NotNil(t, response)
	require.NotNil(t, response.Content)
	require.Contains(t, response.Content, "application/json")
	assert.Equal(t, User{ID: 1, Name: "John"}, response.Content["application/json"].Example)
}

func TestBuilder_MultipleContentTypes(t *testing.T) {
	t.Parallel()

	type CreateUserRequest struct {
		Name string `json:"name"`
	}

	builder := newTestBuilder(t)

	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodPost, Path: "/users"},
			Doc: &RouteDoc{
				RequestType:     reflect.TypeOf(CreateUserRequest{}),
				RequestMetadata: schema.IntrospectRequest(reflect.TypeOf(CreateUserRequest{})),
				Consumes:        []string{"application/json", "application/xml"},
				Produces:        []string{"application/json", "text/xml"},
				ResponseTypes:   map[int]reflect.Type{http.StatusCreated: reflect.TypeOf(User{})},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/users"]
	require.NotNil(t, pathItem.Post)
	require.NotNil(t, pathItem.Post.RequestBody)

	// Should use first content type
	assert.Contains(t, pathItem.Post.RequestBody.Content, "application/json")

	response := pathItem.Post.Responses["201"]
	require.NotNil(t, response)
	require.NotNil(t, response.Content)
	// Should use first produce type
	assert.Contains(t, response.Content, "application/json")
}

func BenchmarkBuilder_Build(b *testing.B) {
	builder := newTestBuilder(b)
	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/users/:id"},
			Doc: &RouteDoc{
				Summary: "Get user",
				ResponseTypes: map[int]reflect.Type{
					http.StatusOK: reflect.TypeOf(User{}),
				},
			},
		},
		{
			RouteInfo: RouteInfo{Method: http.MethodPost, Path: "/users"},
			Doc: &RouteDoc{
				Summary:       "Create user",
				RequestType:   reflect.TypeOf(User{}),
				ResponseTypes: map[int]reflect.Type{http.StatusCreated: reflect.TypeOf(User{})},
			},
		},
		{
			RouteInfo: RouteInfo{Method: http.MethodPut, Path: "/users/:id"},
			Doc: &RouteDoc{
				Summary:       "Update user",
				RequestType:   reflect.TypeOf(User{}),
				ResponseTypes: map[int]reflect.Type{http.StatusOK: reflect.TypeOf(User{})},
			},
		},
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = builder.Build(routes)
	}
}

func BenchmarkBuilder_ComplexSpec(b *testing.B) {
	builder := newTestBuilder(b)
	routes := make([]EnrichedRoute, 0, 50)
	for i := range 50 {
		routes = append(routes, EnrichedRoute{
			RouteInfo: RouteInfo{
				Method: http.MethodGet,
				Path:   fmt.Sprintf("/resource%d/:id", i),
			},
			Doc: &RouteDoc{
				Summary: fmt.Sprintf("Get resource %d", i),
				ResponseTypes: map[int]reflect.Type{
					http.StatusOK: reflect.TypeOf(User{}),
				},
			},
		})
	}

	b.ResetTimer()
	for b.Loop() {
		_, _ = builder.Build(routes)
	}
}

func TestBuilder_ResponseNamedExamples(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)
	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/users/:id"},
			Doc: &RouteDoc{
				ResponseTypes: map[int]reflect.Type{
					http.StatusOK: reflect.TypeOf(User{}),
				},
				ResponseNamedExamples: map[int][]ExampleData{
					http.StatusOK: {
						{
							Name:    "regular",
							Summary: "Regular user",
							Value:   User{ID: 1, Name: "John"},
						},
						{
							Name:        "admin",
							Summary:     "Admin user",
							Description: "User with admin privileges",
							Value:       User{ID: 2, Name: "Admin"},
						},
					},
				},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/users/{id}"]
	require.NotNil(t, pathItem.Get)

	response := pathItem.Get.Responses["200"]
	require.NotNil(t, response)
	require.NotNil(t, response.Content)
	require.Contains(t, response.Content, "application/json")

	mt := response.Content["application/json"]
	require.NotNil(t, mt.Examples)

	assert.Len(t, mt.Examples, 2)
	assert.Contains(t, mt.Examples, "regular")
	assert.Contains(t, mt.Examples, "admin")
	assert.Equal(t, "Regular user", mt.Examples["regular"].Summary)
	assert.Equal(t, "User with admin privileges", mt.Examples["admin"].Description)
}

func TestBuilder_RequestNamedExamples(t *testing.T) {
	t.Parallel()

	type CreateUser struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	builder := newTestBuilder(t)
	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodPost, Path: "/users"},
			Doc: &RouteDoc{
				RequestType:     reflect.TypeOf(CreateUser{}),
				RequestMetadata: schema.IntrospectRequest(reflect.TypeOf(CreateUser{})),
				RequestNamedExamples: []ExampleData{
					{
						Name:    "minimal",
						Summary: "Required fields",
						Value:   CreateUser{Name: "John"},
					},
					{
						Name:    "complete",
						Summary: "All fields",
						Value:   CreateUser{Name: "John", Email: "john@example.com"},
					},
				},
				ResponseTypes: map[int]reflect.Type{http.StatusCreated: reflect.TypeOf(User{})},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	pathItem := spec.Paths["/users"]
	require.NotNil(t, pathItem.Post)
	require.NotNil(t, pathItem.Post.RequestBody)
	require.Contains(t, pathItem.Post.RequestBody.Content, "application/json")

	mt := pathItem.Post.RequestBody.Content["application/json"]
	require.NotNil(t, mt.Examples)

	assert.Len(t, mt.Examples, 2)
	assert.Contains(t, mt.Examples, "minimal")
	assert.Contains(t, mt.Examples, "complete")
}

func TestBuilder_ExternalExample(t *testing.T) {
	t.Parallel()

	builder := newTestBuilder(t)
	routes := []EnrichedRoute{
		{
			RouteInfo: RouteInfo{Method: http.MethodGet, Path: "/data"},
			Doc: &RouteDoc{
				ResponseTypes: map[int]reflect.Type{
					http.StatusOK: reflect.TypeOf(map[string]any{}),
				},
				ResponseNamedExamples: map[int][]ExampleData{
					http.StatusOK: {
						{
							Name:          "large",
							Summary:       "Large dataset",
							ExternalValue: "https://example.com/large.json",
						},
					},
				},
			},
		},
	}

	spec, err := builder.Build(routes)
	require.NoError(t, err)

	mt := spec.Paths["/data"].Get.Responses["200"].Content["application/json"]
	require.NotNil(t, mt.Examples)

	assert.Equal(t, "https://example.com/large.json", mt.Examples["large"].ExternalValue)
	assert.Nil(t, mt.Examples["large"].Value)
}
