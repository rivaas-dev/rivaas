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

package compression

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

//nolint:paralleltest // Tests compression behavior
func TestCompression_BasicGzip(t *testing.T) {
	r := router.MustNew()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "Hello, World!"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))

	// Verify response is actually gzipped
	gr, err := gzip.NewReader(w.Body)
	require.NoError(t, err, "Failed to create gzip reader")
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err, "Failed to decompress response")
	assert.Contains(t, string(decompressed), "Hello, World!")
}

//nolint:paralleltest // Tests compression behavior
func TestCompression_NoGzipSupport(t *testing.T) {
	r := router.MustNew()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "Hello"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Don't set Accept-Encoding header
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.NotEqual(t, "gzip", w.Header().Get("Content-Encoding"), "Should not compress when client doesn't support gzip")
	assert.Contains(t, w.Body.String(), "Hello", "Response should be uncompressed")
}

//nolint:paralleltest // Subtests share router state
func TestCompression_ExcludePaths(t *testing.T) {
	r := router.MustNew()
	r.Use(New(WithExcludePaths("/metrics", "/health")))

	r.GET("/metrics", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"metrics": "data"})
	})

	r.GET("/api", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"api": "response"})
	})

	tests := []struct {
		name               string
		path               string
		shouldBeCompressed bool
	}{
		{"excluded /metrics", "/metrics", false},
		{"non-excluded /api", "/api", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if tt.shouldBeCompressed {
				assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
			} else {
				assert.NotEqual(t, "gzip", w.Header().Get("Content-Encoding"))
			}
		})
	}
}

//nolint:paralleltest // Subtests share router state
func TestCompression_ExcludeExtensions(t *testing.T) {
	r := router.MustNew()
	r.Use(New(WithExcludeExtensions(".jpg", ".png", ".zip")))

	r.GET("/image.jpg", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"type": "fake image data"})
	})

	r.GET("/data.json", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"data": "value"})
	})

	tests := []struct {
		name               string
		path               string
		shouldBeCompressed bool
	}{
		{"excluded .jpg", "/image.jpg", false},
		{"non-excluded .json", "/data.json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if tt.shouldBeCompressed {
				assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
			} else {
				assert.NotEqual(t, "gzip", w.Header().Get("Content-Encoding"))
			}
		})
	}
}

//nolint:paralleltest // Subtests share router state
func TestCompression_ExcludeContentTypes(t *testing.T) {
	r := router.MustNew()
	r.Use(New(WithExcludeContentTypes("image/jpeg", "application/zip")))

	r.GET("/image", func(c *router.Context) {
		// Set content type BEFORE writing response
		c.Response.Header().Set("Content-Type", "image/jpeg")
		// Explicitly call WriteHeader to trigger compression check
		c.Response.WriteHeader(http.StatusOK)
		c.Response.Write([]byte(`{"type": "image data"}`))
	})

	r.GET("/json", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"data": "value"})
	})

	tests := []struct {
		name               string
		path               string
		shouldBeCompressed bool
	}{
		{"excluded image/jpeg", "/image", false},
		{"non-excluded json", "/json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if tt.shouldBeCompressed {
				assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))
			} else {
				assert.NotEqual(t, "gzip", w.Header().Get("Content-Encoding"))
			}
		})
	}
}

//nolint:paralleltest // Subtests share test state
func TestCompression_CompressionLevels(t *testing.T) {
	levels := []int{
		gzip.NoCompression,
		gzip.BestSpeed,
		gzip.DefaultCompression,
		gzip.BestCompression,
	}

	for _, level := range levels {
		t.Run(fmt.Sprintf("level-%d", level), func(t *testing.T) {
			r := router.MustNew()
			r.Use(New(WithGzipLevel(level)))
			r.GET("/test", func(c *router.Context) {
				data := strings.Repeat("compress this ", 100)
				c.JSON(http.StatusOK, map[string]string{"data": data})
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// All levels should set the encoding header
			if level != gzip.NoCompression {
				assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"), "Level %d should set gzip encoding", level)
			}
		})
	}
}

//nolint:paralleltest // Tests compression behavior
func TestCompression_LargeResponse(t *testing.T) {
	r := router.MustNew()
	r.Use(New())

	largeData := strings.Repeat("This is a large response that should be compressed. ", 1000)

	r.GET("/large", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"data": largeData})
	})

	req := httptest.NewRequest(http.MethodGet, "/large", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"))

	// Compressed size should be significantly smaller
	compressedSize := w.Body.Len()
	originalSize := len(largeData)

	assert.Less(t, compressedSize, originalSize, "Compressed size should be smaller than original")

	// Verify decompression works
	gr, err := gzip.NewReader(w.Body)
	require.NoError(t, err, "Failed to create gzip reader")
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	require.NoError(t, err, "Failed to decompress")
	assert.Contains(t, string(decompressed), "This is a large response")
}

//nolint:paralleltest // Tests compression behavior
func TestCompression_MultipleRequests(t *testing.T) {
	r := router.MustNew()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "test"})
	})

	// Make multiple requests to verify pool reuse works
	for i := range 10 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		assert.Equal(t, "gzip", w.Header().Get("Content-Encoding"), "Request %d should be compressed", i)
		assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i)
	}
}

//nolint:paralleltest // Tests compression behavior
func TestCompression_ContentLengthRemoved(t *testing.T) {
	r := router.MustNew()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.Response.Header().Set("Content-Length", "100")
		c.JSON(http.StatusOK, map[string]string{"data": "test response"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Content-Length should be removed when compressing
	assert.NotEqual(t, "100", w.Header().Get("Content-Length"))
}
