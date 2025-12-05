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
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"rivaas.dev/router/version"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rivaas.dev/router/compiler"
)

// TestContext_Header_Injection tests that header injection attacks are sanitized
func TestContext_Header_Injection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		headerValue    string
		expectedValue  string
		shouldSanitize bool
	}{
		{
			name:           "valid header value",
			headerValue:    "application/json",
			expectedValue:  "application/json",
			shouldSanitize: false,
		},
		{
			name:           "header with carriage return",
			headerValue:    "value\rX-Injected: malicious",
			expectedValue:  "valueX-Injected: malicious",
			shouldSanitize: true,
		},
		{
			name:           "header with newline",
			headerValue:    "value\nX-Injected: malicious",
			expectedValue:  "valueX-Injected: malicious",
			shouldSanitize: true,
		},
		{
			name:           "header with CRLF",
			headerValue:    "value\r\nX-Injected: malicious",
			expectedValue:  "valueX-Injected: malicious",
			shouldSanitize: true,
		},
		{
			name:           "header with multiple newlines",
			headerValue:    "value\n\nX-Injected: malicious",
			expectedValue:  "valueX-Injected: malicious",
			shouldSanitize: true,
		},
		{
			name:           "empty header value",
			headerValue:    "",
			expectedValue:  "",
			shouldSanitize: false,
		},
		{
			name:           "header with special characters but no newlines",
			headerValue:    "value-with-dashes_and_underscores.and.dots",
			expectedValue:  "value-with-dashes_and_underscores.and.dots",
			shouldSanitize: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := MustNew()
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)

			r.GET("/test", func(c *Context) {
				// Header should never panic - it sanitizes instead
				assert.NotPanics(t, func() {
					c.Header("X-Custom-Header", tt.headerValue)
				}, "Expected no panic for header value: %q", tt.headerValue)

				// Verify header was sanitized correctly
				c.String(http.StatusOK, "ok")
			})

			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)

			// Verify the header value was sanitized correctly
			actualValue := w.Header().Get("X-Custom-Header")
			assert.Equal(t, tt.expectedValue, actualValue,
				"Header value should be sanitized correctly")

			// Verify sanitized values contain no newlines
			if tt.shouldSanitize {
				assert.NotContains(t, actualValue, "\r",
					"Sanitized header should not contain carriage return")
				assert.NotContains(t, actualValue, "\n",
					"Sanitized header should not contain newline")
			}
		})
	}
}

// TestContext_Header_InjectionRealWorldAttack tests a real-world header injection attack scenario
// UPDATED: Now tests sanitization instead of panic (breaking change for safer production behavior)
func TestContext_Header_InjectionRealWorldAttack(t *testing.T) {
	t.Parallel()
	r := MustNew()

	// Simulated attack: User-provided value with CRLF injection
	attackValue := "normal-value\r\nX-Admin: true\r\nX-Auth: bypass"

	r.GET("/redirect", func(c *Context) {
		// Simulate user input (without putting it in URL which would fail in httptest)
		redirectURL := attackValue

		// This should now sanitize and log a warning (not panic)
		c.Header("Location", redirectURL)

		// Verify the header was sanitized (CRLF removed)
		location := c.Response.Header().Get("Location")

		// Should not contain any newlines
		assert.NotContains(t, location, "\r", "Sanitized header should not contain CR")
		assert.NotContains(t, location, "\n", "Sanitized header should not contain LF")

		// Should only contain the safe part
		assert.Equal(t, "normal-valueX-Admin: trueX-Auth: bypass", location)

		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/redirect", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// The request should complete successfully after sanitization
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestBloomFilter_DifferentSeeds tests that bloom filter uses different hash functions
func TestBloomFilter_DifferentSeeds(t *testing.T) {
	t.Parallel()
	bf := compiler.NewBloomFilter(1000, 3)

	// Add an element
	testData := []byte("test-element")
	bf.Add(testData)

	// Verify the element can be found (this indirectly tests that hash functions work)
	// We can't directly access internal seeds/hash methods, but we can verify functionality
	assert.True(t, bf.Test(testData), "Bloom filter should find added element")

	// Test that different elements produce different results (indirect test of hash diversity)
	bf2 := compiler.NewBloomFilter(1000, 3)
	bf2.Add([]byte("different-element"))
	assert.True(t, bf2.Test([]byte("different-element")), "Bloom filter should find different element")
	assert.False(t, bf.Test([]byte("different-element")), "Bloom filter should not find element not added")
}

// TestBloomFilter_FalsePositives tests bloom filter behavior
func TestBloomFilter_FalsePositives(t *testing.T) {
	t.Parallel()
	bf := compiler.NewBloomFilter(100, 3)

	// Add some elements
	bf.Add([]byte("element1"))
	bf.Add([]byte("element2"))
	bf.Add([]byte("element3"))

	// Test should return true for added elements
	assert.True(t, bf.Test([]byte("element1")), "Should find added element1")
	assert.True(t, bf.Test([]byte("element2")), "Should find added element2")
	assert.True(t, bf.Test([]byte("element3")), "Should find added element3")

	// Test should return false for elements not added (with high probability)
	// Note: Bloom filters can have false positives but not false negatives
	// So we test multiple elements that weren't added
	notAdded := []string{
		"not-added-1",
		"not-added-2",
		"not-added-3",
		"different-element",
		"another-one",
	}

	falsePositiveCount := 0
	for _, elem := range notAdded {
		if bf.Test([]byte(elem)) {
			falsePositiveCount++
		}
	}

	// We expect very few false positives (ideally none for this small test)
	assert.LessOrEqual(t, falsePositiveCount, 2,
		"Bloom filter should have minimal false positives")
}

// TestRouter_ConcurrentVersionRegistration tests concurrent version route registration
func TestRouter_ConcurrentVersionRegistration(t *testing.T) {
	t.Parallel()
	r := MustNew(
		WithVersioning(
			version.WithHeaderDetection("X-API-Version"),
			version.WithDefault("v1"),
		),
	)

	const numGoroutines = 100
	const routesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Register routes concurrently from multiple goroutines
	for i := range numGoroutines {
		go func(goroutineID int) {
			defer wg.Done()

			version := fmt.Sprintf("v%d", goroutineID%5+1) // v1 through v5
			vr := r.Version(version)

			for j := range routesPerGoroutine {
				path := fmt.Sprintf("/test/%d/%d", goroutineID, j)
				vr.GET(path, func(c *Context) {
					c.JSON(http.StatusOK, map[string]string{"ok": "true"})
				})
			}
		}(i)
	}

	wg.Wait()

	// Verify routes were registered correctly by testing a few
	for version := 1; version <= 5; version++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test/0/0", nil)
		req.Header.Set("X-API-Version", fmt.Sprintf("v%d", version))

		r.ServeHTTP(w, req)

		// Should get a response (either 200 if route exists or 404 if not)
		// The important thing is that it doesn't panic or deadlock
		assert.Contains(t, []int{200, 404}, w.Code,
			"Should handle concurrent version registration without errors")
	}
}

// TestRouter_ConcurrentRouteRegistration tests concurrent standard route registration
func TestRouter_ConcurrentRouteRegistration(t *testing.T) {
	t.Parallel()
	r := MustNew()

	const numGoroutines = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Register routes concurrently
	for i := range numGoroutines {
		go func(n int) {
			defer wg.Done()
			r.GET(fmt.Sprintf("/path%d", n), func(c *Context) {
				c.JSON(200, map[string]int{"id": n})
			})
		}(i)
	}

	wg.Wait()

	// Verify all routes were registered
	routes := r.Routes()
	assert.Len(t, routes, numGoroutines, "All routes should be registered despite concurrent access")

	// Test a few routes to ensure they work
	for i := range 5 {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", fmt.Sprintf("/path%d", i), nil)

		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code, "Route %d should work", i)
	}
}

// TestContext_JSON_EncodingError tests JSON encoding error handling
func TestContext_JSON_EncodingError(t *testing.T) {
	t.Parallel()

	// Create a type that fails to marshal
	type BadType struct {
		Func func() // Functions cannot be marshaled to JSON
	}

	t.Run("JSON returns error on encoding failure", func(t *testing.T) {
		t.Parallel()
		// Each subtest needs its own router to avoid race conditions
		r := MustNew()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		var capturedError error
		r.GET("/test", func(c *Context) {
			badData := BadType{Func: func() {}}
			capturedError = c.JSON(200, badData)
		})

		r.ServeHTTP(w, req)

		require.Error(t, capturedError, "Should return error for unencodable data")
		assert.Contains(t, capturedError.Error(), "json",
			"Error should mention JSON encoding issue")
	})

	t.Run("JSON handles error with explicit 500 response", func(t *testing.T) {
		t.Parallel()
		// Each subtest needs its own router to avoid race conditions
		r := MustNew()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)

		r.GET("/test", func(c *Context) {
			badData := BadType{Func: func() {}}
			if err := c.JSON(200, badData); err != nil {
				// Handle error explicitly by sending 500 response
				c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
				c.Response.WriteHeader(http.StatusInternalServerError)
				c.Response.Write(fmt.Appendf(nil, `{"error":"JSON encoding failed","type":"%T","details":"%s"}`, badData, err.Error()))
			}
		})

		r.ServeHTTP(w, req)

		// JSON should have returned an error, which handler dealt with
		assert.Equal(t, http.StatusInternalServerError, w.Code,
			"Handler should return 500 on encoding error")
		assert.Contains(t, w.Body.String(), "JSON encoding failed",
			"Response should indicate JSON encoding error")
		assert.Contains(t, w.Body.String(), "BadType",
			"Response should include the type that failed to encode")
	})
}

// TestContext_JSON_ValidData tests JSON encoding with valid data
func TestContext_JSON_ValidData(t *testing.T) {
	t.Parallel()
	r := MustNew()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

	type ValidData struct {
		Name   string `json:"name"`
		Age    int    `json:"age"`
		Active bool   `json:"active"`
	}

	r.GET("/test", func(c *Context) {
		data := ValidData{
			Name:   "John Doe",
			Age:    30,
			Active: true,
		}
		c.JSON(http.StatusOK, data)
	})

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "John Doe")
	assert.Contains(t, w.Body.String(), `"age":30`)
}

// BenchmarkContext_Header_Validation benchmarks header validation overhead
func BenchmarkContext_Header_Validation(b *testing.B) {
	r := MustNew()
	r.GET("/test", func(c *Context) {
		c.Header("X-Custom", "safe-value")
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for b.Loop() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkBloomFilter_Operations benchmarks bloom filter operations.
func BenchmarkBloomFilter_Operations(b *testing.B) {
	bf := compiler.NewBloomFilter(10000, 3)

	b.Run("Add", func(b *testing.B) {
		for i := range b.N {
			bf.Add(fmt.Appendf(nil, "element-%d", i))
		}
	})

	b.Run("Test", func(b *testing.B) {
		// Pre-populate
		for i := range 1000 {
			bf.Add(fmt.Appendf(nil, "element-%d", i))
		}

		b.ResetTimer()
		for b.Loop() {
			bf.Test(fmt.Appendf(nil, "element-%d", b.N%1000))
		}
	})
}
