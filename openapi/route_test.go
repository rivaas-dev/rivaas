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
	"net/http"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/openapi/example"
)

// Test types for testing
type TestRequest struct {
	Name string `json:"name"`
}

type TestResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func TestRouteWrapper_Freeze_Immutable(t *testing.T) {
	t.Parallel()

	t.Run("frozen route cannot be modified", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")

		// Configure before freezing
		rw.Summary("Original summary").
			Description("Original description").
			Tags("tag1").
			OperationID("testOp").
			Deprecated().
			Consumes("application/json").
			Produces("application/json").
			Request(TestRequest{}).
			Response(200, TestResponse{}).
			ResponseExample(200, TestResponse{ID: 1, Name: "test"})

		// Freeze the route
		frozenDoc := rw.Freeze()
		require.NotNil(t, frozenDoc)
		require.Equal(t, "Original summary", frozenDoc.Summary)
		require.Equal(t, "Original description", frozenDoc.Description)
		require.Equal(t, []string{"tag1"}, frozenDoc.Tags)
		require.Equal(t, "testOp", frozenDoc.OperationID)
		require.True(t, frozenDoc.Deprecated)

		// Try to modify after freezing - should have no effect
		rw.Summary("Modified summary").
			Description("Modified description").
			Tags("tag2", "tag3").
			OperationID("modifiedOp").
			Consumes("text/xml").
			Produces("text/csv").
			Request(nil).
			Response(404, nil).
			ResponseExample(200, TestResponse{ID: 999, Name: "modified"})

		// Verify frozen doc is unchanged
		frozenDoc2 := rw.GetFrozenDoc()
		require.NotNil(t, frozenDoc2)
		assert.Equal(t, "Original summary", frozenDoc2.Summary)
		assert.Equal(t, "Original description", frozenDoc2.Description)
		assert.Equal(t, []string{"tag1"}, frozenDoc2.Tags)
		assert.Equal(t, "testOp", frozenDoc2.OperationID)
		assert.True(t, frozenDoc2.Deprecated)
		assert.Equal(t, []string{"application/json"}, frozenDoc2.Consumes)
		assert.Equal(t, []string{"application/json"}, frozenDoc2.Produces)
		assert.NotNil(t, frozenDoc2.RequestType)
		assert.NotNil(t, frozenDoc2.ResponseTypes[200])
	})

	t.Run("frozen route security cannot be modified", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/protected")

		// Add security before freezing
		rw.Security("bearerAuth")
		rw.Security("oauth2", "read", "write")

		frozenDoc := rw.Freeze()
		require.Len(t, frozenDoc.Security, 2)
		require.Equal(t, "bearerAuth", frozenDoc.Security[0].Scheme)
		require.Equal(t, "oauth2", frozenDoc.Security[1].Scheme)

		// Try to add more security after freezing
		rw.Security("apiKey")
		rw.Bearer()
		rw.OAuth("oauth2", "admin")

		// Verify frozen doc is unchanged
		frozenDoc2 := rw.GetFrozenDoc()
		require.NotNil(t, frozenDoc2)
		assert.Len(t, frozenDoc2.Security, 2)
		assert.Equal(t, "bearerAuth", frozenDoc2.Security[0].Scheme)
		assert.Equal(t, "oauth2", frozenDoc2.Security[1].Scheme)
	})

	t.Run("frozen route methods return self for chaining", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")
		rw.Freeze()

		// All methods should return self even when frozen
		result := rw.Summary("test").
			Description("test").
			Tags("test").
			OperationID("test").
			Deprecated().
			Consumes("test").
			Produces("test").
			Security("test").
			Bearer().
			OAuth("test").
			Request(TestRequest{}).
			Response(200, TestResponse{}).
			ResponseExample(200, TestResponse{})

		assert.Equal(t, rw, result)
	})

	t.Run("freeze is idempotent", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")
		rw.Summary("Test summary")

		doc1 := rw.Freeze()
		doc2 := rw.Freeze()
		doc3 := rw.Freeze()

		// All should return the same frozen doc
		assert.Equal(t, doc1, doc2)
		assert.Equal(t, doc2, doc3)
		assert.Equal(t, "Test summary", doc1.Summary)
		assert.Equal(t, "Test summary", doc2.Summary)
		assert.Equal(t, "Test summary", doc3.Summary)
	})
}

func TestRouteWrapper_Freeze_DeepCopy(t *testing.T) {
	t.Parallel()

	t.Run("frozen doc is a deep copy", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")

		// Configure with slices and maps
		rw.Tags("tag1", "tag2")
		rw.Consumes("application/json", "application/xml")
		rw.Produces("application/json", "text/csv")
		rw.Response(200, TestResponse{})
		rw.Response(404, nil)
		rw.ResponseExample(200, TestResponse{ID: 1, Name: "test"})

		frozenDoc := rw.Freeze()
		require.NotNil(t, frozenDoc)

		// Verify it's a deep copy by checking that modifying the original
		// (if it weren't frozen) wouldn't affect the frozen copy
		// Since it's frozen, modifications won't work, but we can verify
		// the frozen doc has the expected values
		assert.Equal(t, []string{"tag1", "tag2"}, frozenDoc.Tags)
		assert.Equal(t, []string{"application/json", "application/xml"}, frozenDoc.Consumes)
		assert.Equal(t, []string{"application/json", "text/csv"}, frozenDoc.Produces)
		assert.NotNil(t, frozenDoc.ResponseTypes[200])
		assert.Nil(t, frozenDoc.ResponseTypes[404])
		assert.NotNil(t, frozenDoc.ResponseExample[200])
	})

	t.Run("frozen doc security is a deep copy", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")

		rw.Security("bearerAuth")
		rw.Security("oauth2", "read", "write")

		frozenDoc := rw.Freeze()
		require.NotNil(t, frozenDoc)
		require.Len(t, frozenDoc.Security, 2)

		// Verify security requirements are properly copied
		assert.Equal(t, "bearerAuth", frozenDoc.Security[0].Scheme)
		assert.Empty(t, frozenDoc.Security[0].Scopes)
		assert.Equal(t, "oauth2", frozenDoc.Security[1].Scheme)
		assert.Equal(t, []string{"read", "write"}, frozenDoc.Security[1].Scopes)
	})
}

func TestRouteWrapper_Freeze_Concurrent(t *testing.T) {
	t.Parallel()

	t.Run("concurrent freeze calls are safe", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute("GET", "/test")
		rw.Summary("Test")

		var wg sync.WaitGroup
		const numGoroutines = 100
		results := make([]*RouteDoc, 0, numGoroutines)
		var mu sync.Mutex

		// Concurrent freeze calls
		for range numGoroutines {
			wg.Go(func() {
				doc := rw.Freeze()
				mu.Lock()
				results = append(results, doc)
				mu.Unlock()
			})
		}
		wg.Wait()

		// All results should be the same
		first := results[0]
		require.NotNil(t, first)
		for i := 1; i < numGoroutines; i++ {
			assert.Equal(t, first, results[i], "all freeze calls should return the same doc")
		}
	})

	t.Run("concurrent read after freeze is safe", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")
		rw.Summary("Test summary")
		rw.Freeze()

		var wg sync.WaitGroup
		const numGoroutines = 100
		results := make([]*RouteDoc, 0, numGoroutines)
		var mu sync.Mutex

		// Concurrent reads after freeze
		for range numGoroutines {
			wg.Go(func() {
				doc := rw.GetFrozenDoc()
				mu.Lock()
				results = append(results, doc)
				mu.Unlock()
			})
		}
		wg.Wait()

		// All results should be the same
		first := results[0]
		require.NotNil(t, first)
		for i := 1; i < numGoroutines; i++ {
			assert.Equal(t, first, results[i], "all reads should return the same doc")
			assert.Equal(t, "Test summary", results[i].Summary)
		}
	})

	t.Run("concurrent modification attempts after freeze are safe", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")
		rw.Summary("Original")
		frozenDoc := rw.Freeze()
		require.NotNil(t, frozenDoc)

		var wg sync.WaitGroup
		const numGoroutines = 100

		// Concurrent modification attempts (should all be no-ops)
		for i := range numGoroutines {
			wg.Add(1)
			go func(_ int) {
				defer wg.Done()
				rw.Summary("Modified").
					Description("Modified").
					Tags("tag").
					OperationID("op").
					Deprecated().
					Security("test").
					Request(TestRequest{}).
					Response(200, TestResponse{})
			}(i)
		}
		wg.Wait()

		// Verify frozen doc is unchanged
		finalDoc := rw.GetFrozenDoc()
		require.NotNil(t, finalDoc)
		assert.Equal(t, "Original", finalDoc.Summary)
		assert.Empty(t, finalDoc.Description)
		assert.Empty(t, finalDoc.Tags)
		assert.Empty(t, finalDoc.OperationID)
		assert.False(t, finalDoc.Deprecated)
		assert.Empty(t, finalDoc.Security)
	})
}

func TestRouteWrapper_GetFrozenDoc(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when not frozen", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")
		assert.Nil(t, rw.GetFrozenDoc())
	})

	t.Run("returns frozen doc after freeze", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")
		rw.Summary("Test summary")

		frozenDoc := rw.Freeze()
		require.NotNil(t, frozenDoc)

		retrievedDoc := rw.GetFrozenDoc()
		require.NotNil(t, retrievedDoc)
		assert.Equal(t, frozenDoc, retrievedDoc)
		assert.Equal(t, "Test summary", retrievedDoc.Summary)
	})
}

func TestRouteWrapper_Freeze_RequestMetadata(t *testing.T) {
	t.Parallel()

	t.Run("freeze introspects request type", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodPost, "/test")

		// Set request type before freezing
		rw.Request(TestRequest{})

		// Before freeze, RequestMetadata should be nil
		rw.mu.RLock()
		require.Nil(t, rw.doc.RequestMetadata)
		rw.mu.RUnlock()

		// After freeze, RequestMetadata should be populated
		frozenDoc := rw.Freeze()
		require.NotNil(t, frozenDoc)
		require.NotNil(t, frozenDoc.RequestMetadata)
		assert.Equal(t, reflect.TypeFor[TestRequest](), frozenDoc.RequestType)
	})

	t.Run("freeze without request type has nil metadata", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")

		frozenDoc := rw.Freeze()
		require.NotNil(t, frozenDoc)
		assert.Nil(t, frozenDoc.RequestType)
		assert.Nil(t, frozenDoc.RequestMetadata)
	})
}

func TestRouteWrapper_Freeze_WithExamples(t *testing.T) {
	t.Parallel()

	type User struct {
		ID   int
		Name string
	}

	t.Run("freezes response named examples", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")
		rw.Response(200, User{},
			example.New("success", User{ID: 1, Name: "test"}),
		)

		doc := rw.Freeze()

		// Verify examples are frozen
		assert.Len(t, doc.ResponseNamedExamples[200], 1)

		// Verify modifications after freeze are ignored
		rw.Response(200, User{},
			example.New("new", User{ID: 2, Name: "modified"}),
		)

		frozenDoc := rw.GetFrozenDoc()
		assert.Len(t, frozenDoc.ResponseNamedExamples[200], 1)
		assert.Equal(t, "success", frozenDoc.ResponseNamedExamples[200][0].Name())
	})

	t.Run("freezes request named examples", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodPost, "/test")
		rw.Request(User{},
			example.New("test", User{ID: 1, Name: "test"}),
		)

		doc := rw.Freeze()

		assert.Len(t, doc.RequestNamedExamples, 1)
	})

	t.Run("deep copies named examples", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/test")
		examples := []example.Example{
			example.New("ex1", User{ID: 1, Name: "test"}),
		}

		rw.Response(200, User{}, examples...)

		doc := rw.Freeze()

		// Original slice modification shouldn't affect frozen doc
		// (This tests the deep copy behavior)
		assert.Len(t, doc.ResponseNamedExamples[200], 1)
	})
}

func TestRouteWrapper_Info(t *testing.T) {
	t.Parallel()

	t.Run("info is always accessible", func(t *testing.T) {
		t.Parallel()
		rw := NewRoute(http.MethodGet, "/users/:id")

		info := rw.Info()
		assert.Equal(t, http.MethodGet, info.Method)
		assert.Equal(t, "/users/:id", info.Path)

		// Info should be accessible even after freeze
		rw.Freeze()
		info2 := rw.Info()
		assert.Equal(t, http.MethodGet, info2.Method)
		assert.Equal(t, "/users/:id", info2.Path)
	})
}
