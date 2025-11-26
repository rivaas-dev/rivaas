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

package openapi

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/openapi/example"
)

// Test types for testing
type ErrorResponse struct {
	Error string `json:"error"`
}

type DifferentResponse struct {
	Message string `json:"message"`
}

func TestRouteWrapper_Doc(t *testing.T) {
	t.Parallel()

	t.Run("sets both summary and description", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		result := rw.Doc("Test Summary", "Test Description")

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify both fields are set
		rw.mu.RLock()
		assert.Equal(t, "Test Summary", rw.doc.Summary)
		assert.Equal(t, "Test Description", rw.doc.Description)
		rw.mu.RUnlock()
	})

	t.Run("handles empty strings", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Doc("", "")

		rw.mu.RLock()
		assert.Equal(t, "", rw.doc.Summary)
		assert.Equal(t, "", rw.doc.Description)
		rw.mu.RUnlock()
	})

	t.Run("replaces previous values", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Summary("Old Summary").Description("Old Description")
		rw.Doc("New Summary", "New Description")

		rw.mu.RLock()
		assert.Equal(t, "New Summary", rw.doc.Summary)
		assert.Equal(t, "New Description", rw.doc.Description)
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Doc("Original Summary", "Original Description")
		rw.Freeze()

		rw.Doc("Modified Summary", "Modified Description")

		doc := rw.GetFrozenDoc()
		assert.Equal(t, "Original Summary", doc.Summary)
		assert.Equal(t, "Original Description", doc.Description)
	})
}

func TestRouteWrapper_Summary(t *testing.T) {
	t.Parallel()

	t.Run("sets summary correctly", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		result := rw.Summary("Test Summary")

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify summary is set
		rw.mu.RLock()
		assert.Equal(t, "Test Summary", rw.doc.Summary)
		rw.mu.RUnlock()
	})

	t.Run("replaces previous summary", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Summary("First")
		rw.Summary("Second")

		rw.mu.RLock()
		assert.Equal(t, "Second", rw.doc.Summary)
		rw.mu.RUnlock()
	})

	t.Run("handles empty string", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Summary("Initial").Summary("")

		rw.mu.RLock()
		assert.Equal(t, "", rw.doc.Summary)
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Summary("Original")
		rw.Freeze()

		rw.Summary("Modified")

		doc := rw.GetFrozenDoc()
		assert.Equal(t, "Original", doc.Summary)
	})
}

func TestRouteWrapper_Description(t *testing.T) {
	t.Parallel()

	t.Run("sets description correctly", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		result := rw.Description("Test Description")

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify description is set
		rw.mu.RLock()
		assert.Equal(t, "Test Description", rw.doc.Description)
		rw.mu.RUnlock()
	})

	t.Run("replaces previous description", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Description("First")
		rw.Description("Second")

		rw.mu.RLock()
		assert.Equal(t, "Second", rw.doc.Description)
		rw.mu.RUnlock()
	})

	t.Run("handles empty string", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Description("Initial").Description("")

		rw.mu.RLock()
		assert.Equal(t, "", rw.doc.Description)
		rw.mu.RUnlock()
	})

	t.Run("supports markdown formatting", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		markdown := "This is a **bold** description with `code` and [links](http://example.com)"
		rw.Description(markdown)

		rw.mu.RLock()
		assert.Equal(t, markdown, rw.doc.Description)
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Description("Original")
		rw.Freeze()

		rw.Description("Modified")

		doc := rw.GetFrozenDoc()
		assert.Equal(t, "Original", doc.Description)
	})
}

func TestRouteWrapper_Tags(t *testing.T) {
	t.Parallel()

	t.Run("adds single tag", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Tags("users")

		rw.mu.RLock()
		assert.Equal(t, []string{"users"}, rw.doc.Tags)
		rw.mu.RUnlock()
	})

	t.Run("adds multiple tags at once", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Tags("users", "management", "admin")

		rw.mu.RLock()
		assert.Equal(t, []string{"users", "management", "admin"}, rw.doc.Tags)
		rw.mu.RUnlock()
	})

	t.Run("accumulates tags from multiple calls", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Tags("users")
		rw.Tags("management")
		rw.Tags("admin")

		rw.mu.RLock()
		assert.Equal(t, []string{"users", "management", "admin"}, rw.doc.Tags)
		rw.mu.RUnlock()
	})

	t.Run("handles empty call", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Tags("initial")
		rw.Tags() // Should be no-op

		rw.mu.RLock()
		assert.Equal(t, []string{"initial"}, rw.doc.Tags)
		rw.mu.RUnlock()
	})

	t.Run("handles duplicate tags", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Tags("users")
		rw.Tags("users", "admin")
		rw.Tags("users")

		rw.mu.RLock()
		// Tags are accumulated, duplicates are allowed
		assert.Contains(t, rw.doc.Tags, "users")
		assert.Contains(t, rw.doc.Tags, "admin")
		rw.mu.RUnlock()
	})

	t.Run("returns self for chaining", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		result := rw.Tags("tag1")

		assert.Equal(t, rw, result)
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Tags("original")
		rw.Freeze()

		rw.Tags("modified")

		doc := rw.GetFrozenDoc()
		assert.Equal(t, []string{"original"}, doc.Tags)
	})
}

func TestRouteWrapper_OperationID(t *testing.T) {
	t.Parallel()

	t.Run("sets operation ID correctly", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		result := rw.OperationID("getTest")

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify operation ID is set
		rw.mu.RLock()
		assert.Equal(t, "getTest", rw.doc.OperationID)
		rw.mu.RUnlock()
	})

	t.Run("replaces previous operation ID", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.OperationID("first")
		rw.OperationID("second")

		rw.mu.RLock()
		assert.Equal(t, "second", rw.doc.OperationID)
		rw.mu.RUnlock()
	})

	t.Run("handles empty string", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.OperationID("initial").OperationID("")

		rw.mu.RLock()
		assert.Equal(t, "", rw.doc.OperationID)
		rw.mu.RUnlock()
	})

	t.Run("handles camelCase IDs", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/users/:id")

		rw.OperationID("getUserById")

		rw.mu.RLock()
		assert.Equal(t, "getUserById", rw.doc.OperationID)
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.OperationID("original")
		rw.Freeze()

		rw.OperationID("modified")

		doc := rw.GetFrozenDoc()
		assert.Equal(t, "original", doc.OperationID)
	})
}

func TestRouteWrapper_Deprecated(t *testing.T) {
	t.Parallel()

	t.Run("sets deprecated flag to true", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		result := rw.Deprecated()

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify deprecated is set
		rw.mu.RLock()
		assert.True(t, rw.doc.Deprecated)
		rw.mu.RUnlock()
	})

	t.Run("starts as false", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.mu.RLock()
		assert.False(t, rw.doc.Deprecated)
		rw.mu.RUnlock()
	})

	t.Run("multiple calls still set to true", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Deprecated().Deprecated().Deprecated()

		rw.mu.RLock()
		assert.True(t, rw.doc.Deprecated)
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Deprecated()
		rw.Freeze()

		doc := rw.GetFrozenDoc()
		assert.True(t, doc.Deprecated)

		// Try to set again (should be no-op)
		rw.Deprecated()
		doc2 := rw.GetFrozenDoc()
		assert.True(t, doc2.Deprecated)
	})
}

func TestRouteWrapper_Consumes(t *testing.T) {
	t.Parallel()

	t.Run("sets content types", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")

		result := rw.Consumes("application/json", "application/xml")

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify content types are set
		rw.mu.RLock()
		assert.Equal(t, []string{"application/json", "application/xml"}, rw.doc.Consumes)
		rw.mu.RUnlock()
	})

	t.Run("replaces previous content types", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")

		rw.Consumes("application/json")
		rw.Consumes("multipart/form-data", "text/plain")

		rw.mu.RLock()
		assert.Equal(t, []string{"multipart/form-data", "text/plain"}, rw.doc.Consumes)
		rw.mu.RUnlock()
	})

	t.Run("handles single content type", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")

		rw.Consumes("application/json")

		rw.mu.RLock()
		assert.Equal(t, []string{"application/json"}, rw.doc.Consumes)
		rw.mu.RUnlock()
	})

	t.Run("handles empty call", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")

		rw.Consumes("application/json")
		rw.Consumes() // Should replace with empty slice

		rw.mu.RLock()
		assert.Empty(t, rw.doc.Consumes)
		rw.mu.RUnlock()
	})

	t.Run("defaults to application/json", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")

		rw.mu.RLock()
		assert.Equal(t, []string{"application/json"}, rw.doc.Consumes)
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")
		rw.Consumes("application/json")
		rw.Freeze()

		rw.Consumes("text/xml")

		doc := rw.GetFrozenDoc()
		assert.Equal(t, []string{"application/json"}, doc.Consumes)
	})
}

func TestRouteWrapper_Produces(t *testing.T) {
	t.Parallel()

	t.Run("sets content types", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		result := rw.Produces("application/json", "text/csv")

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify content types are set
		rw.mu.RLock()
		assert.Equal(t, []string{"application/json", "text/csv"}, rw.doc.Produces)
		rw.mu.RUnlock()
	})

	t.Run("replaces previous content types", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Produces("application/json")
		rw.Produces("text/html", "text/plain")

		rw.mu.RLock()
		assert.Equal(t, []string{"text/html", "text/plain"}, rw.doc.Produces)
		rw.mu.RUnlock()
	})

	t.Run("handles single content type", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Produces("application/json")

		rw.mu.RLock()
		assert.Equal(t, []string{"application/json"}, rw.doc.Produces)
		rw.mu.RUnlock()
	})

	t.Run("handles empty call", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Produces("application/json")
		rw.Produces() // Should replace with empty slice

		rw.mu.RLock()
		assert.Empty(t, rw.doc.Produces)
		rw.mu.RUnlock()
	})

	t.Run("defaults to application/json", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.mu.RLock()
		assert.Equal(t, []string{"application/json"}, rw.doc.Produces)
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Produces("application/json")
		rw.Freeze()

		rw.Produces("text/xml")

		doc := rw.GetFrozenDoc()
		assert.Equal(t, []string{"application/json"}, doc.Produces)
	})
}

func TestRouteWrapper_Security(t *testing.T) {
	t.Parallel()

	t.Run("adds security requirement", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")

		result := rw.Security("bearerAuth")

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify security is added
		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 1)
		assert.Equal(t, "bearerAuth", rw.doc.Security[0].Scheme)
		assert.Empty(t, rw.doc.Security[0].Scopes)
		rw.mu.RUnlock()
	})

	t.Run("adds security requirement with scopes", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")

		rw.Security("oauth2", "read", "write", "admin")

		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 1)
		assert.Equal(t, "oauth2", rw.doc.Security[0].Scheme)
		assert.Equal(t, []string{"read", "write", "admin"}, rw.doc.Security[0].Scopes)
		rw.mu.RUnlock()
	})

	t.Run("accumulates multiple security requirements", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")

		rw.Security("bearerAuth")
		rw.Security("oauth2", "read", "write")
		rw.Security("apiKey")

		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 3)
		assert.Equal(t, "bearerAuth", rw.doc.Security[0].Scheme)
		assert.Equal(t, "oauth2", rw.doc.Security[1].Scheme)
		assert.Equal(t, []string{"read", "write"}, rw.doc.Security[1].Scopes)
		assert.Equal(t, "apiKey", rw.doc.Security[2].Scheme)
		rw.mu.RUnlock()
	})

	t.Run("handles empty scopes", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")

		rw.Security("bearerAuth")

		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 1)
		assert.Equal(t, "bearerAuth", rw.doc.Security[0].Scheme)
		assert.Empty(t, rw.doc.Security[0].Scopes)
		rw.mu.RUnlock()
	})

	t.Run("starts with empty security", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.mu.RLock()
		assert.Empty(t, rw.doc.Security)
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")
		rw.Security("bearerAuth")
		rw.Freeze()

		rw.Security("oauth2", "read")

		doc := rw.GetFrozenDoc()
		require.Len(t, doc.Security, 1)
		assert.Equal(t, "bearerAuth", doc.Security[0].Scheme)
	})
}

func TestRouteWrapper_Bearer(t *testing.T) {
	t.Parallel()

	t.Run("delegates to Security with bearerAuth", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")

		result := rw.Bearer()

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify security is added
		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 1)
		assert.Equal(t, "bearerAuth", rw.doc.Security[0].Scheme)
		assert.Empty(t, rw.doc.Security[0].Scopes)
		rw.mu.RUnlock()
	})

	t.Run("can be called multiple times", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")

		rw.Bearer().Bearer().Bearer()

		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 3)
		for i := range 3 {
			assert.Equal(t, "bearerAuth", rw.doc.Security[i].Scheme)
		}
		rw.mu.RUnlock()
	})

	t.Run("works with other security methods", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")

		rw.Bearer()
		rw.Security("oauth2", "read")

		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 2)
		assert.Equal(t, "bearerAuth", rw.doc.Security[0].Scheme)
		assert.Equal(t, "oauth2", rw.doc.Security[1].Scheme)
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")
		rw.Bearer()
		rw.Freeze()

		rw.Bearer()

		doc := rw.GetFrozenDoc()
		require.Len(t, doc.Security, 1)
		assert.Equal(t, "bearerAuth", doc.Security[0].Scheme)
	})
}

func TestRouteWrapper_OAuth(t *testing.T) {
	t.Parallel()

	t.Run("delegates to Security with scheme and scopes", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")

		result := rw.OAuth("oauth2", "read", "write", "admin")

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify security is added
		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 1)
		assert.Equal(t, "oauth2", rw.doc.Security[0].Scheme)
		assert.Equal(t, []string{"read", "write", "admin"}, rw.doc.Security[0].Scopes)
		rw.mu.RUnlock()
	})

	t.Run("handles no scopes", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")

		rw.OAuth("oauth2")

		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 1)
		assert.Equal(t, "oauth2", rw.doc.Security[0].Scheme)
		assert.Empty(t, rw.doc.Security[0].Scopes)
		rw.mu.RUnlock()
	})

	t.Run("handles custom scheme names", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")

		rw.OAuth("customOAuth", "scope1", "scope2")

		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 1)
		assert.Equal(t, "customOAuth", rw.doc.Security[0].Scheme)
		assert.Equal(t, []string{"scope1", "scope2"}, rw.doc.Security[0].Scopes)
		rw.mu.RUnlock()
	})

	t.Run("can be called multiple times", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")

		rw.OAuth("oauth2", "read")
		rw.OAuth("oauth2", "write")
		rw.OAuth("custom", "admin")

		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 3)
		assert.Equal(t, "oauth2", rw.doc.Security[0].Scheme)
		assert.Equal(t, []string{"read"}, rw.doc.Security[0].Scopes)
		assert.Equal(t, "oauth2", rw.doc.Security[1].Scheme)
		assert.Equal(t, []string{"write"}, rw.doc.Security[1].Scopes)
		assert.Equal(t, "custom", rw.doc.Security[2].Scheme)
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/protected")
		rw.OAuth("oauth2", "read")
		rw.Freeze()

		rw.OAuth("oauth2", "write")

		doc := rw.GetFrozenDoc()
		require.Len(t, doc.Security, 1)
		assert.Equal(t, "oauth2", doc.Security[0].Scheme)
		assert.Equal(t, []string{"read"}, doc.Security[0].Scopes)
	})
}

func TestRouteWrapper_Request(t *testing.T) {
	t.Parallel()

	t.Run("sets request type", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")

		result := rw.Request(TestRequest{})

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify request type is set
		rw.mu.RLock()
		assert.NotNil(t, rw.doc.RequestType)
		assert.Equal(t, reflect.TypeFor[TestRequest](), rw.doc.RequestType)
		rw.mu.RUnlock()
	})

	t.Run("replaces previous request type", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")

		rw.Request(TestRequest{})
		rw.Request(DifferentResponse{})

		rw.mu.RLock()
		assert.Equal(t, reflect.TypeFor[DifferentResponse](), rw.doc.RequestType)
		rw.mu.RUnlock()
	})

	t.Run("handles pointer types", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")

		rw.Request(&TestRequest{})

		rw.mu.RLock()
		assert.NotNil(t, rw.doc.RequestType)
		assert.Equal(t, reflect.TypeFor[*TestRequest](), rw.doc.RequestType)
		rw.mu.RUnlock()
	})

	t.Run("handles primitive types", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")

		rw.Request("")

		rw.mu.RLock()
		assert.Equal(t, reflect.TypeFor[string](), rw.doc.RequestType)
		rw.mu.RUnlock()
	})

	t.Run("handles slice types", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")

		rw.Request([]TestRequest{})

		rw.mu.RLock()
		assert.Equal(t, reflect.TypeFor[[]TestRequest](), rw.doc.RequestType)
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")
		rw.Request(TestRequest{})
		rw.Freeze()

		rw.Request(DifferentResponse{})

		doc := rw.GetFrozenDoc()
		assert.Equal(t, reflect.TypeFor[TestRequest](), doc.RequestType)
	})
}

func TestRouteWrapper_Response(t *testing.T) {
	t.Parallel()

	t.Run("sets response type", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Response(200, TestResponse{})

		rw.mu.RLock()
		assert.NotNil(t, rw.doc.ResponseTypes[200])
		assert.Equal(t, reflect.TypeFor[TestResponse](), rw.doc.ResponseTypes[200])
		rw.mu.RUnlock()
	})

	t.Run("sets nil response for no-content", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("DELETE", "/test")

		rw.Response(204, nil)

		rw.mu.RLock()
		assert.Nil(t, rw.doc.ResponseTypes[204])
		assert.Contains(t, rw.doc.ResponseTypes, 204)
		rw.mu.RUnlock()
	})

	t.Run("allows multiple status codes", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Response(200, TestResponse{})
		rw.Response(404, ErrorResponse{})
		rw.Response(500, ErrorResponse{})

		rw.mu.RLock()
		assert.Len(t, rw.doc.ResponseTypes, 3)
		assert.NotNil(t, rw.doc.ResponseTypes[200])
		assert.NotNil(t, rw.doc.ResponseTypes[404])
		assert.NotNil(t, rw.doc.ResponseTypes[500])
		assert.Equal(t, reflect.TypeFor[TestResponse](), rw.doc.ResponseTypes[200])
		assert.Equal(t, reflect.TypeFor[ErrorResponse](), rw.doc.ResponseTypes[404])
		assert.Equal(t, reflect.TypeFor[ErrorResponse](), rw.doc.ResponseTypes[500])
		rw.mu.RUnlock()
	})

	t.Run("replaces response for same status code", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Response(200, TestResponse{})
		rw.Response(200, DifferentResponse{})

		rw.mu.RLock()
		assert.Equal(t, reflect.TypeFor[DifferentResponse](), rw.doc.ResponseTypes[200])
		rw.mu.RUnlock()
	})

	t.Run("handles pointer types", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Response(200, &TestResponse{})

		rw.mu.RLock()
		assert.Equal(t, reflect.TypeFor[*TestResponse](), rw.doc.ResponseTypes[200])
		rw.mu.RUnlock()
	})

	t.Run("handles primitive types", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Response(200, "")

		rw.mu.RLock()
		assert.Equal(t, reflect.TypeFor[string](), rw.doc.ResponseTypes[200])
		rw.mu.RUnlock()
	})

	t.Run("returns self for chaining", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		result := rw.Response(200, TestResponse{})

		assert.Equal(t, rw, result)
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Response(200, TestResponse{})
		rw.Freeze()

		rw.Response(200, DifferentResponse{})

		doc := rw.GetFrozenDoc()
		assert.Equal(t, reflect.TypeFor[TestResponse](), doc.ResponseTypes[200])
	})
}

func TestRouteWrapper_ResponseExample(t *testing.T) {
	t.Parallel()

	t.Run("sets response example", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		example := TestResponse{ID: 123, Name: "Test"}
		result := rw.ResponseExample(200, example)

		// Should return self for chaining
		assert.Equal(t, rw, result)

		// Verify example is set
		rw.mu.RLock()
		assert.NotNil(t, rw.doc.ResponseExample[200])
		assert.Equal(t, example, rw.doc.ResponseExample[200])
		rw.mu.RUnlock()
	})

	t.Run("allows multiple status codes", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.ResponseExample(200, TestResponse{ID: 1, Name: "Success"})
		rw.ResponseExample(404, ErrorResponse{Error: "Not Found"})
		rw.ResponseExample(500, ErrorResponse{Error: "Internal Error"})

		rw.mu.RLock()
		assert.Len(t, rw.doc.ResponseExample, 3)
		assert.Equal(t, TestResponse{ID: 1, Name: "Success"}, rw.doc.ResponseExample[200])
		assert.Equal(t, ErrorResponse{Error: "Not Found"}, rw.doc.ResponseExample[404])
		assert.Equal(t, ErrorResponse{Error: "Internal Error"}, rw.doc.ResponseExample[500])
		rw.mu.RUnlock()
	})

	t.Run("replaces example for same status code", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.ResponseExample(200, TestResponse{ID: 1, Name: "First"})
		rw.ResponseExample(200, TestResponse{ID: 2, Name: "Second"})

		rw.mu.RLock()
		assert.Equal(t, TestResponse{ID: 2, Name: "Second"}, rw.doc.ResponseExample[200])
		rw.mu.RUnlock()
	})

	t.Run("handles nil example", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.ResponseExample(204, nil)

		rw.mu.RLock()
		assert.Nil(t, rw.doc.ResponseExample[204])
		assert.Contains(t, rw.doc.ResponseExample, 204)
		rw.mu.RUnlock()
	})

	t.Run("handles primitive examples", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.ResponseExample(200, "string example")
		rw.ResponseExample(201, 42)

		rw.mu.RLock()
		assert.Equal(t, "string example", rw.doc.ResponseExample[200])
		assert.Equal(t, 42, rw.doc.ResponseExample[201])
		rw.mu.RUnlock()
	})

	t.Run("no-op after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.ResponseExample(200, TestResponse{ID: 1, Name: "Original"})
		rw.Freeze()

		rw.ResponseExample(200, TestResponse{ID: 2, Name: "Modified"})

		doc := rw.GetFrozenDoc()
		assert.Equal(t, TestResponse{ID: 1, Name: "Original"}, doc.ResponseExample[200])
	})
}

func TestRouteWrapper_MethodChaining(t *testing.T) {
	t.Parallel()

	t.Run("all methods can be chained", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/users")

		result := rw.
			Doc("Create User", "Creates a new user account").
			Tags("users", "management").
			OperationID("createUser").
			Deprecated().
			Consumes("application/json").
			Produces("application/json").
			Bearer().
			Request(TestRequest{}).
			Response(201, TestResponse{}).
			Response(400, ErrorResponse{}).
			ResponseExample(201, TestResponse{ID: 1, Name: "New User"})

		// Should return the RouteWrapper for chaining
		assert.Equal(t, rw, result)

		// Verify all values are set
		rw.mu.RLock()
		assert.Equal(t, "Create User", rw.doc.Summary)
		assert.Equal(t, "Creates a new user account", rw.doc.Description)
		assert.Equal(t, []string{"users", "management"}, rw.doc.Tags)
		assert.Equal(t, "createUser", rw.doc.OperationID)
		assert.True(t, rw.doc.Deprecated)
		assert.Equal(t, []string{"application/json"}, rw.doc.Consumes)
		assert.Equal(t, []string{"application/json"}, rw.doc.Produces)
		require.Len(t, rw.doc.Security, 1)
		assert.Equal(t, "bearerAuth", rw.doc.Security[0].Scheme)
		assert.Equal(t, reflect.TypeFor[TestRequest](), rw.doc.RequestType)
		assert.NotNil(t, rw.doc.ResponseTypes[201])
		assert.NotNil(t, rw.doc.ResponseTypes[400])
		assert.Equal(t, TestResponse{ID: 1, Name: "New User"}, rw.doc.ResponseExample[201])
		rw.mu.RUnlock()
	})

	t.Run("complex chaining scenario", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/users/:id")

		rw.
			Summary("Get User").
			Description("Retrieves a user by ID").
			Tags("users").
			Tags("read").
			OperationID("getUserById").
			Bearer().
			OAuth("oauth2", "read").
			Response(200, TestResponse{}).
			Response(404, ErrorResponse{}).
			Response(500, ErrorResponse{}).
			ResponseExample(200, TestResponse{ID: 123, Name: "John Doe"}).
			ResponseExample(404, ErrorResponse{Error: "User not found"})

		rw.mu.RLock()
		assert.Equal(t, "Get User", rw.doc.Summary)
		assert.Equal(t, []string{"users", "read"}, rw.doc.Tags)
		require.Len(t, rw.doc.Security, 2)
		assert.Len(t, rw.doc.ResponseTypes, 3)
		assert.Len(t, rw.doc.ResponseExample, 2)
		rw.mu.RUnlock()
	})
}

func TestRouteWrapper_AccumulationBehavior(t *testing.T) {
	t.Parallel()

	t.Run("tags accumulate", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Tags("tag1")
		rw.Tags("tag2", "tag3")
		rw.Tags("tag4")

		rw.mu.RLock()
		assert.Equal(t, []string{"tag1", "tag2", "tag3", "tag4"}, rw.doc.Tags)
		rw.mu.RUnlock()
	})

	t.Run("security accumulates", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Security("scheme1")
		rw.Security("scheme2", "scope1")
		rw.Bearer()
		rw.OAuth("oauth2", "read")

		rw.mu.RLock()
		require.Len(t, rw.doc.Security, 4)
		assert.Equal(t, "scheme1", rw.doc.Security[0].Scheme)
		assert.Equal(t, "scheme2", rw.doc.Security[1].Scheme)
		assert.Equal(t, "bearerAuth", rw.doc.Security[2].Scheme)
		assert.Equal(t, "oauth2", rw.doc.Security[3].Scheme)
		rw.mu.RUnlock()
	})

	t.Run("responses accumulate", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Response(200, TestResponse{})
		rw.Response(404, ErrorResponse{})
		rw.Response(500, ErrorResponse{})

		rw.mu.RLock()
		assert.Len(t, rw.doc.ResponseTypes, 3)
		rw.mu.RUnlock()
	})

	t.Run("response examples accumulate", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.ResponseExample(200, TestResponse{ID: 1})
		rw.ResponseExample(404, ErrorResponse{Error: "Not Found"})
		rw.ResponseExample(500, ErrorResponse{Error: "Error"})

		rw.mu.RLock()
		assert.Len(t, rw.doc.ResponseExample, 3)
		rw.mu.RUnlock()
	})

	t.Run("summary replaces", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Summary("First")
		rw.Summary("Second")

		rw.mu.RLock()
		assert.Equal(t, "Second", rw.doc.Summary)
		rw.mu.RUnlock()
	})

	t.Run("description replaces", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Description("First")
		rw.Description("Second")

		rw.mu.RLock()
		assert.Equal(t, "Second", rw.doc.Description)
		rw.mu.RUnlock()
	})

	t.Run("operation ID replaces", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.OperationID("first")
		rw.OperationID("second")

		rw.mu.RLock()
		assert.Equal(t, "second", rw.doc.OperationID)
		rw.mu.RUnlock()
	})

	t.Run("consumes replaces", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/test")

		rw.Consumes("application/json")
		rw.Consumes("text/xml")

		rw.mu.RLock()
		assert.Equal(t, []string{"text/xml"}, rw.doc.Consumes)
		rw.mu.RUnlock()
	})

	t.Run("produces replaces", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")

		rw.Produces("application/json")
		rw.Produces("text/csv")

		rw.mu.RLock()
		assert.Equal(t, []string{"text/csv"}, rw.doc.Produces)
		rw.mu.RUnlock()
	})
}

func TestRouteWrapper_CompleteWorkflow(t *testing.T) {
	t.Parallel()

	t.Run("realistic API endpoint configuration", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("PUT", "/users/:id")

		// Configure the route
		rw.
			Doc("Update User", "Updates an existing user by ID. Requires authentication.").
			Tags("users", "management").
			OperationID("updateUser").
			Consumes("application/json").
			Produces("application/json").
			Bearer().
			Request(TestRequest{}).
			Response(200, TestResponse{}).
			Response(400, ErrorResponse{}).
			Response(404, ErrorResponse{}).
			Response(500, ErrorResponse{}).
			ResponseExample(200, TestResponse{ID: 123, Name: "Updated User"}).
			ResponseExample(400, ErrorResponse{Error: "Invalid input"}).
			ResponseExample(404, ErrorResponse{Error: "User not found"})

		// Verify configuration
		rw.mu.RLock()
		assert.Equal(t, "Update User", rw.doc.Summary)
		assert.Equal(t, "Updates an existing user by ID. Requires authentication.", rw.doc.Description)
		assert.Equal(t, []string{"users", "management"}, rw.doc.Tags)
		assert.Equal(t, "updateUser", rw.doc.OperationID)
		assert.Equal(t, []string{"application/json"}, rw.doc.Consumes)
		assert.Equal(t, []string{"application/json"}, rw.doc.Produces)
		require.Len(t, rw.doc.Security, 1)
		assert.Equal(t, "bearerAuth", rw.doc.Security[0].Scheme)
		assert.Equal(t, reflect.TypeFor[TestRequest](), rw.doc.RequestType)
		assert.Len(t, rw.doc.ResponseTypes, 4)
		assert.Len(t, rw.doc.ResponseExample, 3)
		rw.mu.RUnlock()

		// Freeze and verify
		frozenDoc := rw.Freeze()
		require.NotNil(t, frozenDoc)
		assert.Equal(t, "Update User", frozenDoc.Summary)
		assert.Len(t, frozenDoc.ResponseTypes, 4)
		assert.Len(t, frozenDoc.ResponseExample, 3)

		// Verify modifications after freeze are ignored
		rw.Summary("Modified")
		rw.Tags("modified")
		finalDoc := rw.GetFrozenDoc()
		assert.Equal(t, "Update User", finalDoc.Summary)
		assert.Equal(t, []string{"users", "management"}, finalDoc.Tags)
	})
}

func TestIsZeroValue(t *testing.T) {
	t.Parallel()

	type testStruct struct {
		ID   int
		Name string
	}

	tests := []struct {
		name  string
		value any
		want  bool
	}{
		// Nil cases
		{name: "nil", value: nil, want: true},
		{name: "nil pointer", value: (*int)(nil), want: true},
		{name: "nil slice", value: []int(nil), want: true},
		{name: "nil map", value: map[string]int(nil), want: true},
		// Zero value primitives
		{name: "zero int", value: 0, want: true},
		{name: "zero string", value: "", want: true},
		{name: "zero float", value: 0.0, want: true},
		{name: "zero bool", value: false, want: true},
		// Non-zero primitives
		{name: "non-zero int", value: 1, want: false},
		{name: "non-zero string", value: "hello", want: false},
		{name: "non-zero float", value: 1.5, want: false},
		{name: "true bool", value: true, want: false},
		// Structs
		{name: "zero struct", value: testStruct{}, want: true},
		{name: "partial struct", value: testStruct{ID: 1}, want: false},
		{name: "full struct", value: testStruct{ID: 1, Name: "test"}, want: false},
		// Slices
		{name: "empty slice", value: []int{}, want: false}, // Empty slice is NOT nil
		{name: "non-empty slice", value: []int{1, 2}, want: false},
		// Maps
		{name: "empty map", value: map[string]int{}, want: false}, // Empty map is NOT nil
		{name: "non-empty map", value: map[string]int{"a": 1}, want: false},
		// Pointers
		{name: "pointer to zero", value: new(int), want: false}, // Pointer itself is not nil
		{name: "pointer to value", value: func() *int { v := 5; return &v }(), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isZeroValue(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRouteWrapper_Response_WithNamedExamples(t *testing.T) {
	t.Parallel()

	type UserResponse struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	t.Run("single named example", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Response(200, UserResponse{},
			example.New("success", UserResponse{ID: 123, Name: "John"}),
		)

		rw.mu.RLock()
		defer rw.mu.RUnlock()

		assert.NotNil(t, rw.doc.ResponseTypes[200])
		assert.Len(t, rw.doc.ResponseNamedExamples[200], 1)
		assert.Empty(t, rw.doc.ResponseExample)
		assert.Equal(t, "success", rw.doc.ResponseNamedExamples[200][0].Name())
	})

	t.Run("multiple named examples", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Response(200, UserResponse{},
			example.New("regular", UserResponse{ID: 1, Name: "User"}),
			example.New("admin", UserResponse{ID: 2, Name: "Admin"},
				example.WithSummary("Admin user")),
		)

		rw.mu.RLock()
		defer rw.mu.RUnlock()

		assert.Len(t, rw.doc.ResponseNamedExamples[200], 2)
		assert.Equal(t, "regular", rw.doc.ResponseNamedExamples[200][0].Name())
		assert.Equal(t, "admin", rw.doc.ResponseNamedExamples[200][1].Name())
		assert.Equal(t, "Admin user", rw.doc.ResponseNamedExamples[200][1].Summary())
	})

	t.Run("named examples clear single example", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		// First set a single example
		rw.Response(200, UserResponse{ID: 1, Name: "Single"})

		assert.NotEmpty(t, rw.doc.ResponseExample)

		// Then set named examples
		rw.Response(200, UserResponse{},
			example.New("named", UserResponse{ID: 2}),
		)

		rw.mu.RLock()
		defer rw.mu.RUnlock()

		assert.Empty(t, rw.doc.ResponseExample[200])
		assert.Len(t, rw.doc.ResponseNamedExamples[200], 1)
	})

	t.Run("single example clears named examples", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		// First set named examples
		rw.Response(200, UserResponse{},
			example.New("named", UserResponse{ID: 1}),
		)

		assert.Len(t, rw.doc.ResponseNamedExamples[200], 1)

		// Then set single example
		rw.Response(200, UserResponse{ID: 2, Name: "Single"})

		rw.mu.RLock()
		defer rw.mu.RUnlock()

		assert.Empty(t, rw.doc.ResponseNamedExamples[200])
		assert.NotNil(t, rw.doc.ResponseExample[200])
	})

	t.Run("zero value without examples has no example", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Response(200, UserResponse{}) // Zero value

		rw.mu.RLock()
		defer rw.mu.RUnlock()

		assert.NotNil(t, rw.doc.ResponseTypes[200])
		assert.Empty(t, rw.doc.ResponseExample[200])
	})

	t.Run("non-zero value becomes example", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Response(200, UserResponse{ID: 123, Name: "John"})

		rw.mu.RLock()
		defer rw.mu.RUnlock()

		assert.Equal(t, UserResponse{ID: 123, Name: "John"}, rw.doc.ResponseExample[200])
	})

	t.Run("external example", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Response(200, UserResponse{},
			example.NewExternal("large", "https://example.com/large.json",
				example.WithSummary("Large dataset")),
		)

		rw.mu.RLock()
		defer rw.mu.RUnlock()

		assert.Len(t, rw.doc.ResponseNamedExamples[200], 1)
		assert.True(t, rw.doc.ResponseNamedExamples[200][0].IsExternal())
		assert.Equal(t, "https://example.com/large.json",
			rw.doc.ResponseNamedExamples[200][0].ExternalValue())
	})
}

func TestRouteWrapper_Request_WithNamedExamples(t *testing.T) {
	t.Parallel()

	type CreateUserRequest struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	t.Run("single named example", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/users")
		rw.Request(CreateUserRequest{},
			example.New("basic", CreateUserRequest{Name: "John"}),
		)

		rw.mu.RLock()
		defer rw.mu.RUnlock()

		assert.NotNil(t, rw.doc.RequestType)
		assert.Len(t, rw.doc.RequestNamedExamples, 1)
		assert.Nil(t, rw.doc.RequestExample)
	})

	t.Run("multiple named examples", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/users")
		rw.Request(CreateUserRequest{},
			example.New("minimal", CreateUserRequest{Name: "John"},
				example.WithSummary("Required fields only")),
			example.New("complete", CreateUserRequest{Name: "John", Email: "john@example.com"},
				example.WithSummary("All fields")),
		)

		rw.mu.RLock()
		defer rw.mu.RUnlock()

		assert.Len(t, rw.doc.RequestNamedExamples, 2)
		assert.Equal(t, "minimal", rw.doc.RequestNamedExamples[0].Name())
		assert.Equal(t, "complete", rw.doc.RequestNamedExamples[1].Name())
	})

	t.Run("zero value without examples has no example", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/users")
		rw.Request(CreateUserRequest{})

		rw.mu.RLock()
		defer rw.mu.RUnlock()

		assert.NotNil(t, rw.doc.RequestType)
		assert.Nil(t, rw.doc.RequestExample)
	})

	t.Run("non-zero value becomes example", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("POST", "/users")
		rw.Request(CreateUserRequest{Name: "John", Email: "john@example.com"})

		rw.mu.RLock()
		defer rw.mu.RUnlock()

		assert.Equal(t, CreateUserRequest{Name: "John", Email: "john@example.com"},
			rw.doc.RequestExample)
	})
}
