package compression

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"rivaas.dev/router"
)

func TestCompression_BasicGzip(t *testing.T) {
	r := router.New()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "Hello, World!"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Expected Content-Encoding: gzip")
	}

	// Verify response is actually gzipped
	gr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("Failed to decompress response: %v", err)
	}

	if !strings.Contains(string(decompressed), "Hello, World!") {
		t.Errorf("Decompressed response should contain original data, got: %s", string(decompressed))
	}
}

func TestCompression_NoGzipSupport(t *testing.T) {
	r := router.New()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "Hello"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Don't set Accept-Encoding header
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Should not compress when client doesn't support gzip")
	}

	// Response should be uncompressed
	if strings.Contains(w.Body.String(), "Hello") {
		// Good - response is not compressed
	} else {
		t.Error("Response should be uncompressed")
	}
}

func TestCompression_MinSize(t *testing.T) {
	t.Skip("MinSize feature requires response buffering - not yet implemented for performance reasons")

	// TODO: Implement minSize by buffering responses up to a certain size
	// This adds latency and memory overhead, so needs careful consideration
	// For now, compression always occurs if client supports it and content is not excluded
}

func TestCompression_ExcludePaths(t *testing.T) {
	r := router.New()
	r.Use(New(WithExcludePaths("/metrics", "/health")))

	r.GET("/metrics", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"metrics": "data"})
	})

	r.GET("/api", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"api": "response"})
	})

	// Excluded path should not be compressed
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Excluded path should not be compressed")
	}

	// Non-excluded path should be compressed
	req = httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Non-excluded path should be compressed")
	}
}

func TestCompression_ExcludeExtensions(t *testing.T) {
	r := router.New()
	r.Use(New(WithExcludeExtensions(".jpg", ".png", ".zip")))

	r.GET("/image.jpg", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"type": "fake image data"})
	})

	r.GET("/data.json", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"data": "value"})
	})

	// Image should not be compressed
	req := httptest.NewRequest(http.MethodGet, "/image.jpg", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Image file should not be compressed")
	}

	// JSON should be compressed
	req = httptest.NewRequest(http.MethodGet, "/data.json", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("JSON file should be compressed")
	}
}

func TestCompression_ExcludeContentTypes(t *testing.T) {
	r := router.New()
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

	// Image content type should not be compressed
	req := httptest.NewRequest(http.MethodGet, "/image", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Image content type should not be compressed")
	}

	// JSON should be compressed
	req = httptest.NewRequest(http.MethodGet, "/json", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("JSON should be compressed")
	}
}

func TestCompression_CompressionLevels(t *testing.T) {
	levels := []int{
		gzip.NoCompression,
		gzip.BestSpeed,
		gzip.DefaultCompression,
		gzip.BestCompression,
	}

	for _, level := range levels {
		t.Run(fmt.Sprintf("level-%d", level), func(t *testing.T) {
			r := router.New()
			r.Use(New(WithLevel(level)))
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
				if w.Header().Get("Content-Encoding") != "gzip" {
					t.Errorf("Expected Content-Encoding: gzip for level %d", level)
				}
			}
		})
	}
}

func TestCompression_LargeResponse(t *testing.T) {
	r := router.New()
	r.Use(New())

	largeData := strings.Repeat("This is a large response that should be compressed. ", 1000)

	r.GET("/large", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"data": largeData})
	})

	req := httptest.NewRequest(http.MethodGet, "/large", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Large response should be compressed")
	}

	// Compressed size should be significantly smaller
	compressedSize := w.Body.Len()
	originalSize := len(largeData)

	if compressedSize >= originalSize {
		t.Errorf("Compressed size (%d) should be smaller than original (%d)", compressedSize, originalSize)
	}

	// Verify decompression works
	gr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	decompressed, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	if !strings.Contains(string(decompressed), "This is a large response") {
		t.Error("Decompressed data should contain original content")
	}
}

func TestCompression_MultipleRequests(t *testing.T) {
	r := router.New()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "test"})
	})

	// Make multiple requests to verify pool reuse works
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Header().Get("Content-Encoding") != "gzip" {
			t.Errorf("Request %d: expected compression", i)
		}

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected status 200, got %d", i, w.Code)
		}
	}
}

func TestCompression_ContentLengthRemoved(t *testing.T) {
	r := router.New()
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
	if w.Header().Get("Content-Length") == "100" {
		t.Error("Content-Length should be removed/updated when compressing")
	}
}

// Benchmark tests
func BenchmarkCompression_Enabled(b *testing.B) {
	r := router.New()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "test data"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCompression_Disabled(b *testing.B) {
	r := router.New()
	r.Use(New())
	r.GET("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "test data"})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No Accept-Encoding header

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCompression_LargeResponse(b *testing.B) {
	r := router.New()
	r.Use(New())

	largeData := strings.Repeat("benchmark data ", 1000)
	r.GET("/large", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"data": largeData})
	})

	req := httptest.NewRequest(http.MethodGet, "/large", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCompression_BestSpeed(b *testing.B) {
	r := router.New()
	r.Use(New(WithLevel(gzip.BestSpeed)))
	r.GET("/test", func(c *router.Context) {
		data := strings.Repeat("data ", 100)
		c.JSON(http.StatusOK, map[string]string{"content": data})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkCompression_BestCompression(b *testing.B) {
	r := router.New()
	r.Use(New(WithLevel(gzip.BestCompression)))
	r.GET("/test", func(c *router.Context) {
		data := strings.Repeat("data ", 100)
		c.JSON(http.StatusOK, map[string]string{"content": data})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
