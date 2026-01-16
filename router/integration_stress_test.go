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

package router_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"rivaas.dev/router"
)

var _ = Describe("Router Stress Tests", func() {
	Describe("Concurrent route registration", func() {
		It("should register routes concurrently without panics", func() {
			r := router.MustNew()

			var wg sync.WaitGroup
			routeCount := 100

			// Register routes from multiple goroutines
			for i := range routeCount {
				id := i
				wg.Go(func() {
					path := "/route" + string(rune('0'+id%10))
					r.GET(path, func(c *router.Context) {
						c.Status(http.StatusOK)
					})
				})
			}

			wg.Wait()

			// Warmup should not panic
			Expect(func() { r.Warmup() }).NotTo(Panic())

			// Routes should work
			req := httptest.NewRequest(http.MethodGet, "/route0", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("Concurrent requests", func() {
		It("should handle concurrent requests correctly", func() {
			r := router.MustNew()

			var requestCount atomic.Int64

			r.GET("/test", func(c *router.Context) {
				requestCount.Add(1)
				time.Sleep(1 * time.Millisecond) // Simulate work
				//nolint:errcheck // Test handler
				c.JSON(http.StatusOK, map[string]int64{"count": requestCount.Load()})
			})

			r.Warmup()

			var wg sync.WaitGroup
			concurrency := 100

			for range concurrency {
				wg.Go(func() {
					req := httptest.NewRequest(http.MethodGet, "/test", nil)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)

					Expect(w.Code).To(Equal(http.StatusOK))
				})
			}

			wg.Wait()

			Expect(requestCount.Load()).To(Equal(int64(concurrency)))
		})
	})

	Describe("Memory leak detection", func() {
		It("should properly recycle contexts without leaks", func() {
			r := router.MustNew()

			r.GET("/test", func(c *router.Context) {
				//nolint:errcheck // Test handler
				c.JSON(http.StatusOK, map[string]string{"status": "ok"})
			})

			r.Warmup()

			// Run many requests
			// This test mainly verifies no panics and contexts are properly recycled
			// Actual memory leak detection would require runtime.MemStats analysis
			for range 10000 {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				Expect(w.Code).To(Equal(http.StatusOK))
			}
		})
	})

	Describe("Large number of routes", func() {
		It("should handle router with many routes", func() {
			r := router.MustNew()

			// Register 1000 routes
			for i := range 1000 {
				path := "/route" + string(rune('0'+i%10)) + "/" + string(rune('a'+i%26))
				r.GET(path, func(c *router.Context) {
					c.Status(http.StatusOK)
				})
			}

			// Warmup
			r.Warmup()

			// Test a route
			req := httptest.NewRequest(http.MethodGet, "/route0/a", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("Context reuse", func() {
		It("should not leak data between requests", func() {
			r := router.MustNew()

			var firstRequestParam string
			var secondRequestParam string

			r.GET("/first/:id", func(c *router.Context) {
				firstRequestParam = c.Param("id")
				c.Param("nonexistent") // Access non-existent param
				c.Status(http.StatusOK)
			})

			r.GET("/second", func(c *router.Context) {
				// Should not have params from first request
				secondRequestParam = c.Param("id")
				c.Status(http.StatusOK)
			})

			// First request
			req1 := httptest.NewRequest(http.MethodGet, "/first/123", nil)
			w1 := httptest.NewRecorder()
			r.ServeHTTP(w1, req1)

			Expect(firstRequestParam).To(Equal("123"))

			// Second request (should use recycled context)
			req2 := httptest.NewRequest(http.MethodGet, "/second", nil)
			w2 := httptest.NewRecorder()
			r.ServeHTTP(w2, req2)

			Expect(secondRequestParam).To(BeEmpty())
		})
	})

	Describe("High load stress test", func() {
		It("should handle high concurrent load", func() {
			r := router.MustNew()

			// Register routes
			for i := range 100 {
				path := "/api/resource" + string(rune('0'+i%10)) + "/:id"
				r.GET(path, func(c *router.Context) {
					//nolint:errcheck // Test handler
					c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
				})
			}

			r.Warmup()

			// High concurrent load
			var wg sync.WaitGroup
			var successCount atomic.Int64
			var errorCount atomic.Int64

			concurrency := 1000
			requestsPerRoutine := 100

			for range concurrency {
				wg.Go(func() {
					for j := range requestsPerRoutine {
						path := "/api/resource" + string(rune('0'+j%10)) + "/123"
						req := httptest.NewRequest(http.MethodGet, path, nil)
						w := httptest.NewRecorder()

						r.ServeHTTP(w, req)

						if w.Code == http.StatusOK {
							successCount.Add(1)
						} else {
							errorCount.Add(1)
						}
					}
				})
			}

			wg.Wait()

			totalExpected := int64(concurrency * requestsPerRoutine)
			actualTotal := successCount.Load() + errorCount.Load()

			Expect(actualTotal).To(Equal(totalExpected))
			Expect(errorCount.Load()).To(Equal(int64(0)))
		})
	})

	Describe("Route compilation with many routes", func() {
		It("should compile and optimize routes efficiently with large number of routes", func() {
			r := router.MustNew()

			// Register 1000+ routes to test compilation performance
			routeCount := 1000
			for i := range routeCount {
				path := "/route" + string(rune('0'+i%10)) + "/" + string(rune('a'+i%26)) + "/" + string(rune('A'+(i/26)%26))
				r.GET(path, func(c *router.Context) {
					//nolint:errcheck // Test handler
					c.String(http.StatusOK, "route")
				})
			}

			// Trigger compilation - should not panic and should complete reasonably fast
			Expect(func() { r.Warmup() }).NotTo(Panic())

			// Test that compiled routes work correctly
			req := httptest.NewRequest(http.MethodGet, "/route0/a/A", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Body.String()).To(Equal("route"))

			// Test multiple routes to verify compilation worked
			for i := range 10 {
				path := "/route" + string(rune('0'+i%10)) + "/" + string(rune('a'+i%26)) + "/" + string(rune('A'+(i/26)%26))
				testReq := httptest.NewRequest(http.MethodGet, path, nil)
				testW := httptest.NewRecorder()
				r.ServeHTTP(testW, testReq)
				Expect(testW.Code).To(Equal(http.StatusOK))
			}
		})
	})

	Describe("Cancellation check performance", func() {
		It("should handle concurrent requests efficiently with cancellation checking enabled", func() {
			for _, enabled := range []bool{true, false} {
				By("testing with cancellation check " + map[bool]string{true: "enabled", false: "disabled"}[enabled])

				r := router.MustNew(router.WithCancellationCheck(enabled))

				// Add multiple middleware to test the check happens in each
				for range 5 {
					r.Use(func(c *router.Context) {
						c.Next()
					})
				}

				r.GET("/test", func(c *router.Context) {
					c.Status(http.StatusOK)
				})

				r.Warmup()

				// Concurrent load test
				var wg sync.WaitGroup
				var successCount atomic.Int64
				concurrency := 500
				requestsPerRoutine := 50

				for range concurrency {
					wg.Go(func() {
						for range requestsPerRoutine {
							req := httptest.NewRequest(http.MethodGet, "/test", nil)
							w := httptest.NewRecorder()
							r.ServeHTTP(w, req)

							if w.Code == http.StatusOK {
								successCount.Add(1)
							}
						}
					})
				}

				wg.Wait()

				totalExpected := int64(concurrency * requestsPerRoutine)
				Expect(successCount.Load()).To(Equal(totalExpected))
			}
		})
	})

	Describe("Bloom filter with large size and concurrent requests", func() {
		It("should handle large bloom filter size with concurrent requests", func() {
			r := router.MustNew(router.WithBloomFilterSize(100000))

			// Register many routes
			for i := range 500 {
				path := "/api/resource" + string(rune('0'+i%10)) + "/" + string(rune('a'+i%26))
				r.GET(path, func(c *router.Context) {
					c.Status(http.StatusOK)
				})
			}

			r.Warmup()

			// Concurrent requests
			var wg sync.WaitGroup
			var successCount atomic.Int64
			concurrency := 200
			requestsPerRoutine := 100

			for i := range concurrency {
				id := i
				wg.Go(func() {
					for j := range requestsPerRoutine {
						path := "/api/resource" + string(rune('0'+(id+j)%10)) + "/" + string(rune('a'+(id+j)%26))
						req := httptest.NewRequest(http.MethodGet, path, nil)
						w := httptest.NewRecorder()
						r.ServeHTTP(w, req)

						if w.Code == http.StatusOK {
							successCount.Add(1)
						}
					}
				})
			}

			wg.Wait()

			totalExpected := int64(concurrency * requestsPerRoutine)
			Expect(successCount.Load()).To(Equal(totalExpected))
		})
	})
})
