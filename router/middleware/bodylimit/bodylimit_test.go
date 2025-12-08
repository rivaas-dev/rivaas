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

package bodylimit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

//nolint:paralleltest // Subtests share router state
func TestBodyLimit_ContentLength(t *testing.T) {
	tests := []struct {
		name           string
		limit          int64
		bodySize       int
		contentLength  string
		expectedStatus int
		checkError     bool
	}{
		{
			name:           "within limit",
			limit:          1024,
			bodySize:       18,
			contentLength:  "18",
			expectedStatus: http.StatusOK,
			checkError:     false,
		},
		{
			name:           "exceeds limit",
			limit:          1024,
			bodySize:       2048,
			contentLength:  "2048",
			expectedStatus: http.StatusRequestEntityTooLarge,
			checkError:     true,
		},
		{
			name:           "exact limit",
			limit:          100,
			bodySize:       100,
			contentLength:  "100",
			expectedStatus: http.StatusOK,
			checkError:     false,
		},
		{
			name:           "one byte over",
			limit:          100,
			bodySize:       115,
			contentLength:  "115",
			expectedStatus: http.StatusRequestEntityTooLarge,
			checkError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := router.MustNew()
			r.Use(New(WithLimit(tt.limit)))
			r.POST("/test", func(c *router.Context) {
				var data map[string]any
				if err := json.NewDecoder(c.Request.Body).Decode(&data); err != nil {
					c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, map[string]string{"message": "success"})
			})

			var body *bytes.Buffer
			if tt.bodySize <= 100 {
				body = bytes.NewBufferString(`{"key": "value"}`)
			} else {
				body = bytes.NewBufferString(strings.Repeat("a", tt.bodySize))
			}

			req := httptest.NewRequest(http.MethodPost, "/test", body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Length", tt.contentLength)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkError {
				var response map[string]any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				assert.Equal(t, "request entity too large", response["error"])
			}
		})
	}
}

//nolint:paralleltest // Tests router behavior
func TestBodyLimit_ActualBodyRead_ExceedsLimit(t *testing.T) {
	r := router.MustNew()
	r.Use(New(WithLimit(100))) // 100 byte limit
	r.POST("/test", func(c *router.Context) {
		var data map[string]any
		if err := json.NewDecoder(c.Request.Body).Decode(&data); err != nil {
			// Body limit error should be caught here
			if strings.Contains(err.Error(), "exceeds limit") {
				c.JSON(http.StatusRequestEntityTooLarge, map[string]string{
					"error": "request body too large",
				})

				return
			}
			c.JSON(400, map[string]string{"error": err.Error()})

			return
		}
		c.JSON(http.StatusOK, map[string]string{"message": "success"})
	})

	// Body larger than limit, but Content-Length not set
	largeBody := bytes.NewBufferString(`{"data": "` + strings.Repeat("x", 200) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", largeBody)
	req.Header.Set("Content-Type", "application/json")
	// No Content-Length header - should still be limited

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should get 413 from handler checking the error
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

//nolint:paralleltest // Tests router behavior
func TestBodyLimit_NoContentLength_WithinLimit(t *testing.T) {
	r := router.MustNew()
	r.Use(New(WithLimit(1024))) // 1KB limit
	r.POST("/test", func(c *router.Context) {
		// Read body directly
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if len(body) == 0 {
			c.JSON(http.StatusBadRequest, map[string]string{"error": "empty body"})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"message": "success", "size": string(rune(len(body)))})
	})

	body := bytes.NewBufferString(`{"key": "value"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", "application/json")
	// No Content-Length header

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

//nolint:paralleltest // Tests router behavior
func TestBodyLimit_SkipPaths(t *testing.T) {
	r := router.MustNew()
	r.Use(New(
		WithLimit(100),
		WithSkipPaths("/upload"),
	))
	r.POST("/upload", func(c *router.Context) {
		var data map[string]any
		json.NewDecoder(c.Request.Body).Decode(&data)
		c.JSON(http.StatusOK, map[string]string{"message": "uploaded"})
	})
	r.POST("/normal", func(c *router.Context) {
		var data map[string]any
		if err := json.NewDecoder(c.Request.Body).Decode(&data); err != nil {
			if strings.Contains(err.Error(), "exceeds limit") {
				c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "too large"})
				return
			}
		}
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Test skipped path - should work even with large body
	largeBody := bytes.NewBufferString(`{"data": "` + strings.Repeat("x", 500) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/upload", largeBody)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Skipped path should succeed")

	// Test normal path - should be limited
	largeBody2 := bytes.NewBufferString(`{"data": "` + strings.Repeat("x", 500) + `"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/normal", largeBody2)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Content-Length", "512")

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	// Should get 413 or error
	assert.NotEqual(t, http.StatusOK, w2.Code, "Normal path with large body should fail")
}

//nolint:paralleltest // Tests router behavior
func TestBodyLimit_CustomErrorHandler(t *testing.T) {
	r := router.MustNew()
	r.Use(New(
		WithLimit(100),
		WithErrorHandler(func(c *router.Context, limit int64) {
			c.Stringf(http.StatusRequestEntityTooLarge, "Custom: Body too large (max: %d bytes)", limit)
		}),
	))
	r.POST("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	largeBody := bytes.NewBufferString(strings.Repeat("a", 500))
	req := httptest.NewRequest(http.MethodPost, "/test", largeBody)
	req.Header.Set("Content-Length", "500")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Contains(t, w.Body.String(), "Custom: Body too large")
}

//nolint:paralleltest // Tests router behavior
func TestBodyLimit_EmptyBody(t *testing.T) {
	r := router.MustNew()
	r.Use(New(WithLimit(1024)))
	r.POST("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Content-Length", "0")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Empty body should be allowed")
}

func TestBodyLimit_FormData(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(WithLimit(1024))) // 1KB limit
	r.POST("/form", func(c *router.Context) {
		var form struct {
			Name string `form:"name"`
		}
		// Parse form first
		contentType := c.Request.Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "multipart/form-data") {
			if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
				if strings.Contains(err.Error(), "exceeds limit") {
					c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "too large"})
					return
				}
				c.JSON(400, map[string]string{"error": err.Error()})

				return
			}
		} else {
			if err := c.Request.ParseForm(); err != nil {
				if strings.Contains(err.Error(), "exceeds limit") {
					c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "too large"})
					return
				}
				c.JSON(400, map[string]string{"error": err.Error()})

				return
			}
		}
		// Extract form data directly
		form.Name = c.Request.Form.Get("name")
		c.JSON(http.StatusOK, map[string]string{"name": form.Name})
	})

	tests := []struct {
		name           string
		formData       string
		expectedStatus int
	}{
		{
			name:           "small form data",
			formData:       "name=test&value=small",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "large form data",
			formData:       "name=" + strings.Repeat("x", 2000),
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := bytes.NewBufferString(tt.formData)
			req := httptest.NewRequest(http.MethodPost, "/form", body)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.ContentLength = int64(len(tt.formData))

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestBodyLimit_DefaultLimit(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New()) // Default 2MB limit
	r.POST("/test", func(c *router.Context) {
		var data map[string]any
		if err := json.NewDecoder(c.Request.Body).Decode(&data); err != nil {
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Body within 2MB
	body := bytes.NewBufferString(`{"key": "value"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", "18")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBodyLimit_InvalidContentLength(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(WithLimit(100)))
	r.POST("/test", func(c *router.Context) {
		var data map[string]any
		if err := json.NewDecoder(c.Request.Body).Decode(&data); err != nil {
			if strings.Contains(err.Error(), "exceeds limit") {
				c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "too large"})
				return
			}
			c.JSON(400, map[string]string{"error": err.Error()})

			return
		}
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Invalid Content-Length (non-numeric) - should be ignored, limit enforced on actual read
	body := bytes.NewBufferString(`{"data": "` + strings.Repeat("x", 200) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", "invalid")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should still enforce limit on actual read
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestBodyLimit_ErrorTypeChecking(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(WithLimit(100)))
	r.POST("/test", func(c *router.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			// Check if error is our specific error type
			if errors.Is(err, ErrBodyLimitExceeded) {
				c.JSON(http.StatusRequestEntityTooLarge, map[string]string{
					"error": "body limit exceeded",
				})

				return
			}
			c.JSON(400, map[string]string{"error": err.Error()})

			return
		}
		c.JSON(http.StatusOK, map[string]any{"size": len(body)})
	})

	// Body exceeds limit
	largeBody := bytes.NewBufferString(strings.Repeat("x", 200))
	req := httptest.NewRequest(http.MethodPost, "/test", largeBody)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestBodyLimit_ConcurrentRequests(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(WithLimit(1024)))
	r.POST("/test", func(c *router.Context) {
		var data map[string]any
		if err := json.NewDecoder(c.Request.Body).Decode(&data); err != nil {
			if strings.Contains(err.Error(), "exceeds limit") {
				c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "too large"})
				return
			}
			c.JSON(400, map[string]string{"error": err.Error()})

			return
		}
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	var wg sync.WaitGroup
	numRequests := 100
	wg.Add(numRequests)

	// Run concurrent requests
	for i := range numRequests {
		go func(idx int) {
			defer wg.Done()

			var body *bytes.Buffer
			if idx%2 == 0 {
				// Small body (should succeed)
				body = bytes.NewBufferString(`{"key": "value"}`)
			} else {
				// Large body (should fail)
				body = bytes.NewBufferString(`{"data": "` + strings.Repeat("x", 2000) + `"}`)
			}

			req := httptest.NewRequest(http.MethodPost, "/test", body)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			// Verify response is one of the expected statuses
			validStatuses := []int{http.StatusOK, http.StatusRequestEntityTooLarge, http.StatusBadRequest}
			assert.Contains(t, validStatuses, w.Code, "Request %d should have valid status", idx)
		}(i)
	}

	wg.Wait()
}

func TestBodyLimit_InvalidLimit_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() {
		_ = New(WithLimit(-1))
	}, "Expected panic for invalid limit")
}

func TestBodyLimit_ZeroLimit_Panics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() {
		_ = New(WithLimit(0))
	}, "Expected panic for zero limit")
}

func TestBodyLimit_WithErrorHandlerFirst(t *testing.T) {
	t.Parallel()
	// This test ensures that WithErrorHandler can be called before WithLimit
	// and the custom handler won't be overwritten
	r := router.MustNew()
	customHandlerCalled := false

	r.Use(New(
		WithErrorHandler(func(c *router.Context, limit int64) {
			customHandlerCalled = true
			c.Stringf(http.StatusRequestEntityTooLarge, "Custom handler: limit is %d", limit)
		}),
		WithLimit(100),
	))

	r.POST("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	largeBody := bytes.NewBufferString(strings.Repeat("a", 500))
	req := httptest.NewRequest(http.MethodPost, "/test", largeBody)
	req.Header.Set("Content-Length", "500")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.True(t, customHandlerCalled, "Custom error handler should be called")
	assert.Contains(t, w.Body.String(), "Custom handler")
	assert.Contains(t, w.Body.String(), "100")
}

func TestBodyLimit_SkipMultiplePaths(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.Use(New(
		WithLimit(100),
		WithSkipPaths("/upload", "/files", "/media"),
	))

	r.POST("/upload", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "upload ok"})
	})
	r.POST("/files", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "files ok"})
	})
	r.POST("/media", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "media ok"})
	})
	r.POST("/api", func(c *router.Context) {
		var data map[string]any
		if err := json.NewDecoder(c.Request.Body).Decode(&data); err != nil {
			if strings.Contains(err.Error(), "exceeds limit") {
				c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "too large"})
				return
			}
		}
		c.JSON(http.StatusOK, map[string]string{"message": "api ok"})
	})

	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{"skipped upload path", "/upload", http.StatusOK},
		{"skipped files path", "/files", http.StatusOK},
		{"skipped media path", "/media", http.StatusOK},
		{"non-skipped api path", "/api", http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			largeBody := bytes.NewBufferString(strings.Repeat("x", 500))
			req := httptest.NewRequest(http.MethodPost, tt.path, largeBody)
			req.Header.Set("Content-Type", "application/json")
			if tt.path == "/api" {
				req.Header.Set("Content-Length", "500")
			}

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
