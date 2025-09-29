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

package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestContextRelease verifies that Release() properly clears data and returns context to pool.
func TestContextRelease(t *testing.T) {
	t.Parallel()

	// Create a context manually to test Release() without router interference
	r := MustNew()
	req := httptest.NewRequest("GET", "/users/123?page=2", nil)
	w := httptest.NewRecorder()

	// Get context from pool manually
	c := r.contextPool.Get(int32(1)) // 1 parameter
	c.initForRequest(req, w, nil, r)
	c.paramKeys[0] = "id"
	c.paramValues[0] = "123"
	c.paramCount = 1

	// Verify context is valid before release
	assert.Equal(t, "123", c.Param("id"))
	assert.Equal(t, "2", c.Query("page"))
	assert.NotNil(t, c.Request)
	assert.NotNil(t, c.Response)

	// Store references before release
	originalRequest := c.Request
	originalResponse := c.Response

	// Release the context
	c.Release()

	// Verify context was cleared (released flag is reset by pool, but data is cleared)
	// The important thing is that sensitive data is cleared
	assert.Nil(t, c.Request, "Request should be cleared after release")
	assert.Nil(t, c.Response, "Response should be cleared after release")
	assert.Equal(t, int32(0), c.paramCount, "paramCount should be reset after release")

	// Verify original references are different (context was reset)
	assert.NotEqual(t, originalRequest, c.Request, "Request should be different after release")
	assert.NotEqual(t, originalResponse, c.Response, "Response should be different after release")
}

// TestContextReleasePreventsUseAfterRelease verifies that released contexts cannot be used.
func TestContextReleasePreventsUseAfterRelease(t *testing.T) {
	t.Parallel()

	// Create a context manually to test Release() without router interference
	r := MustNew()
	req := httptest.NewRequest("GET", "/test?id=123", nil)
	w := httptest.NewRecorder()

	// Get context from pool manually
	c := r.contextPool.Get(int32(1))
	c.initForRequest(req, w, nil, r)
	c.paramKeys[0] = "id"
	c.paramValues[0] = "123"
	c.paramCount = 1

	// Store original values
	originalID := c.Param("id")
	assert.Equal(t, "123", originalID)

	// Release the context
	c.Release()

	// Verify context was cleared
	assert.Nil(t, c.Request, "Request should be cleared after release")
	assert.Nil(t, c.Response, "Response should be cleared after release")

	// Attempt to use released context - should return empty/default values
	assert.Equal(t, "", c.Param("id"), "Param should return empty after release")
	assert.Equal(t, "", c.Query("page"), "Query should return empty after release")

	// Attempt to write response - should return error
	err := c.WriteJSON(http.StatusOK, map[string]string{"test": "value"})
	assert.Error(t, err, "WriteJSON should return error after release")
	assert.Contains(t, err.Error(), "released", "Error should mention release")
}

// TestContextReleaseDoubleRelease verifies that double release is safe.
func TestContextReleaseDoubleRelease(t *testing.T) {
	t.Parallel()

	// Create a context manually
	r := MustNew()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	c := r.contextPool.Get(int32(0))
	c.initForRequest(req, w, nil, r)

	// Store original values
	originalRequest := c.Request
	originalResponse := c.Response

	// Release the context
	c.Release()
	// Attempt double release - should be safe (no panic)
	c.Release()

	// Verify context was cleared (double release is safe)
	assert.Nil(t, c.Request, "Request should be cleared after release")
	assert.Nil(t, c.Response, "Response should be cleared after release")
	assert.NotEqual(t, originalRequest, c.Request, "Request should be different after release")
	assert.NotEqual(t, originalResponse, c.Response, "Response should be different after release")
}

// TestContextReleaseAsyncOperation demonstrates correct usage in async operations.
func TestContextReleaseAsyncOperation(t *testing.T) {
	t.Parallel()

	r := MustNew()
	req := httptest.NewRequest("GET", "/async", nil)
	w := httptest.NewRecorder()

	done := make(chan bool)

	r.GET("/async", func(c *Context) {
		// Start async operation
		go func(ctx *Context) {
			defer ctx.Release() // CRITICAL: Release when done
			// Simulate async work
			_ = ctx.Param("id")
			done <- true
		}(c)
	})

	r.ServeHTTP(w, req)

	// Wait for async operation
	<-done

	// Context should be released by async goroutine
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestContextReleaseNormalUsage verifies that normal usage doesn't require manual release.
func TestContextReleaseNormalUsage(t *testing.T) {
	t.Parallel()

	r := MustNew()
	req := httptest.NewRequest("GET", "/normal", nil)
	w := httptest.NewRecorder()

	r.GET("/normal", func(c *Context) {
		// Normal usage - no manual release needed
		userID := c.Param("id")
		c.JSON(200, map[string]string{"id": userID})
		// Context automatically returned to pool by router
	})

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "id")
}
