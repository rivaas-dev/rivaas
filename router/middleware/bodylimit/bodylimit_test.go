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

	"rivaas.dev/router"
)

func TestBodyLimit_ContentLength_WithinLimit(t *testing.T) {
	r := router.New()
	r.Use(New(WithLimit(1024))) // 1KB limit
	r.POST("/test", func(c *router.Context) {
		var data map[string]interface{}
		if err := c.BindJSON(&data); err != nil {
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"message": "success"})
	})

	body := bytes.NewBufferString(`{"key": "value"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", "18")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestBodyLimit_ContentLength_ExceedsLimit(t *testing.T) {
	r := router.New()
	r.Use(New(WithLimit(1024))) // 1KB limit
	r.POST("/test", func(c *router.Context) {
		var data map[string]interface{}
		if err := c.BindJSON(&data); err != nil {
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"message": "success"})
	})

	// Create body larger than limit
	largeBody := bytes.NewBufferString(strings.Repeat("a", 2048))
	req := httptest.NewRequest(http.MethodPost, "/test", largeBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", "2048")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["error"] != "request entity too large" {
		t.Errorf("Expected error message, got %v", response["error"])
	}
}

func TestBodyLimit_ActualBodyRead_ExceedsLimit(t *testing.T) {
	r := router.New()
	r.Use(New(WithLimit(100))) // 100 byte limit
	r.POST("/test", func(c *router.Context) {
		var data map[string]interface{}
		if err := c.BindJSON(&data); err != nil {
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
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestBodyLimit_NoContentLength_WithinLimit(t *testing.T) {
	r := router.New()
	r.Use(New(WithLimit(1024))) // 1KB limit
	r.POST("/test", func(c *router.Context) {
		// Read body directly
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		if len(body) == 0 {
			c.JSON(400, map[string]string{"error": "empty body"})
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

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestBodyLimit_SkipPaths(t *testing.T) {
	r := router.New()
	r.Use(New(
		WithLimit(100),
		WithSkipPaths("/upload"),
	))
	r.POST("/upload", func(c *router.Context) {
		var data map[string]interface{}
		c.BindJSON(&data)
		c.JSON(http.StatusOK, map[string]string{"message": "uploaded"})
	})
	r.POST("/normal", func(c *router.Context) {
		var data map[string]interface{}
		if err := c.BindJSON(&data); err != nil {
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

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for skipped path, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Test normal path - should be limited
	largeBody2 := bytes.NewBufferString(`{"data": "` + strings.Repeat("x", 500) + `"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/normal", largeBody2)
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Content-Length", "512")

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	// Should get 413 or error
	if w2.Code == http.StatusOK {
		t.Errorf("Expected error status for normal path with large body, got 200")
	}
}

func TestBodyLimit_CustomErrorHandler(t *testing.T) {
	r := router.New()
	r.Use(New(
		WithLimit(100),
		WithErrorHandler(func(c *router.Context, limit int64) {
			c.String(http.StatusRequestEntityTooLarge, "Custom: Body too large (max: %d bytes)", limit)
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

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Custom: Body too large") {
		t.Errorf("Expected custom error message, got: %s", w.Body.String())
	}
}

func TestBodyLimit_EmptyBody(t *testing.T) {
	r := router.New()
	r.Use(New(WithLimit(1024)))
	r.POST("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Content-Length", "0")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for empty body, got %d", w.Code)
	}
}

func TestBodyLimit_ExactLimit(t *testing.T) {
	r := router.New()
	limit := int64(100)
	r.Use(New(WithLimit(limit)))
	r.POST("/test", func(c *router.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]any{
			"message": "ok",
			"size":    len(body),
		})
	})

	// Body exactly at limit
	body := bytes.NewBufferString(strings.Repeat("a", int(limit)))
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Length", "100")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for body at exact limit, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestBodyLimit_OneByteOver(t *testing.T) {
	r := router.New()
	limit := int64(100)
	r.Use(New(WithLimit(limit)))
	r.POST("/test", func(c *router.Context) {
		var data map[string]interface{}
		if err := c.BindJSON(&data); err != nil {
			if strings.Contains(err.Error(), "exceeds limit") {
				c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "too large"})
				return
			}
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	// Body one byte over limit
	body := bytes.NewBufferString(`{"data": "` + strings.Repeat("x", int(limit)) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", "115") // Over limit

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413 for body over limit, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestBodyLimit_FormData(t *testing.T) {
	r := router.New()
	r.Use(New(WithLimit(1024))) // 1KB limit
	r.POST("/form", func(c *router.Context) {
		var form struct {
			Name string `form:"name"`
		}
		if err := c.BindForm(&form); err != nil {
			if strings.Contains(err.Error(), "exceeds limit") {
				c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "too large"})
				return
			}
			c.JSON(400, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"name": form.Name})
	})

	// Small form data
	formData := "name=test&value=small"
	body := bytes.NewBufferString(formData)
	req := httptest.NewRequest(http.MethodPost, "/form", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", "20")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for small form, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Large form data
	largeFormData := "name=" + strings.Repeat("x", 2000)
	largeBody := bytes.NewBufferString(largeFormData)
	req2 := httptest.NewRequest(http.MethodPost, "/form", largeBody)
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.Header.Set("Content-Length", "2005")

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413 for large form, got %d", w2.Code)
	}
}

func TestBodyLimit_DefaultLimit(t *testing.T) {
	r := router.New()
	r.Use(New()) // Default 2MB limit
	r.POST("/test", func(c *router.Context) {
		var data map[string]interface{}
		if err := c.BindJSON(&data); err != nil {
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

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestBodyLimit_InvalidContentLength(t *testing.T) {
	r := router.New()
	r.Use(New(WithLimit(100)))
	r.POST("/test", func(c *router.Context) {
		var data map[string]interface{}
		if err := c.BindJSON(&data); err != nil {
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
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func BenchmarkBodyLimit_ContentLengthCheck(b *testing.B) {
	r := router.New()
	r.Use(New(WithLimit(1024 * 1024)))
	r.POST("/test", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create fresh request for each iteration
		body := bytes.NewBufferString(`{"key": "value"}`)
		req := httptest.NewRequest(http.MethodPost, "/test", body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Length", "18")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkBodyLimit_NoContentLength(b *testing.B) {
	r := router.New()
	r.Use(New(WithLimit(1024 * 1024)))
	r.POST("/test", func(c *router.Context) {
		io.Copy(io.Discard, c.Request.Body)
		c.JSON(http.StatusOK, map[string]string{"message": "ok"})
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create fresh request for each iteration
		body := bytes.NewBufferString(`{"key": "value"}`)
		req := httptest.NewRequest(http.MethodPost, "/test", body)
		req.Header.Set("Content-Type", "application/json")
		// No Content-Length header

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func TestBodyLimit_ErrorTypeChecking(t *testing.T) {
	r := router.New()
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

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestBodyLimit_ConcurrentRequests(t *testing.T) {
	r := router.New()
	r.Use(New(WithLimit(1024)))
	r.POST("/test", func(c *router.Context) {
		var data map[string]interface{}
		if err := c.BindJSON(&data); err != nil {
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
	for i := 0; i < numRequests; i++ {
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
			if w.Code != http.StatusOK && w.Code != http.StatusRequestEntityTooLarge && w.Code != 400 {
				t.Errorf("Request %d: unexpected status %d", idx, w.Code)
			}
		}(i)
	}

	wg.Wait()
}

func TestBodyLimit_InvalidLimit_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid limit, but didn't panic")
		}
	}()

	// This should panic
	_ = New(WithLimit(-1))
}

func TestBodyLimit_ZeroLimit_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for zero limit, but didn't panic")
		}
	}()

	// This should panic
	_ = New(WithLimit(0))
}

func TestBodyLimit_WithErrorHandlerFirst(t *testing.T) {
	// This test ensures that WithErrorHandler can be called before WithLimit
	// and the custom handler won't be overwritten
	r := router.New()
	customHandlerCalled := false

	r.Use(New(
		WithErrorHandler(func(c *router.Context, limit int64) {
			customHandlerCalled = true
			c.String(http.StatusRequestEntityTooLarge, "Custom handler: limit is %d", limit)
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

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d", w.Code)
	}

	if !customHandlerCalled {
		t.Error("Custom error handler was not called")
	}

	if !strings.Contains(w.Body.String(), "Custom handler") {
		t.Errorf("Expected custom handler message, got: %s", w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "100") {
		t.Errorf("Expected limit value in message, got: %s", w.Body.String())
	}
}

func TestBodyLimit_SkipMultiplePaths(t *testing.T) {
	// Test that variadic parameters work with multiple paths
	r := router.New()
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
		var data map[string]interface{}
		if err := c.BindJSON(&data); err != nil {
			if strings.Contains(err.Error(), "exceeds limit") {
				c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "too large"})
				return
			}
		}
		c.JSON(http.StatusOK, map[string]string{"message": "api ok"})
	})

	// Test all skipped paths with large bodies
	skipPaths := []string{"/upload", "/files", "/media"}
	for _, path := range skipPaths {
		largeBody := bytes.NewBufferString(strings.Repeat("x", 500))
		req := httptest.NewRequest(http.MethodPost, path, largeBody)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Path %s: Expected status 200, got %d", path, w.Code)
		}
	}

	// Test non-skipped path with large body (should fail)
	largeBody := bytes.NewBufferString(strings.Repeat("x", 500))
	req := httptest.NewRequest(http.MethodPost, "/api", largeBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", "500")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("/api: Expected status 413, got %d", w.Code)
	}
}
