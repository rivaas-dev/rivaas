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

// Package main provides examples of using the RateLimit middleware.
package main

import (
	"net/http"
	"os"

	"github.com/charmbracelet/log"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware/ratelimit"
)

func main() {
	r := router.MustNew()

	// Example 1: Basic rate limiting
	basicRateLimitExample(r)

	// Example 2: Per-IP rate limiting
	perIPExample(r)

	// Example 3: API key-based rate limiting
	apiKeyExample(r)

	// Example 4: Different limits for different endpoints
	tieredLimitsExample(r)

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	logger.Info("🚀 Server starting on http://localhost:8080")
	logger.Print("")
	logger.Print("📝 Available endpoints:")
	logger.Print("  GET /basic               - 5 requests per second globally")
	logger.Print("  GET /api/public          - 10 req/s per IP")
	logger.Print("  GET /api/premium?key=xyz - 100 req/s with API key")
	logger.Print("  GET /free/data           - Free tier (1 req/s)")
	logger.Print("  GET /pro/data?token=xyz  - Pro tier (20 req/s)")
	logger.Print("")
	logger.Print("📋 Example commands:")
	logger.Print("  # Test global rate limit (send 10 requests quickly)")
	logger.Print("  for i in {1..10}; do curl http://localhost:8080/basic; done")
	logger.Print("")
	logger.Print("  # Test per-IP rate limit")
	logger.Print("  curl http://localhost:8080/api/public")
	logger.Print("")
	logger.Print("  # Test API key rate limit")
	logger.Print("  curl 'http://localhost:8080/api/premium?key=xyz'")
	logger.Print("")
	logger.Print("💡 Tip: Rate limit headers are included in responses:")
	logger.Print("   - X-Rate-Limit-Remaining: remaining requests")
	logger.Print("   - X-Rate-Limit-Reset: seconds until reset")
	logger.Print("")

	logger.Fatal(http.ListenAndServe(":8080", r))
}

// Example 1: Basic global rate limit
func basicRateLimitExample(r *router.Router) {
	r.Use(ratelimit.New(
		ratelimit.WithRequestsPerSecond(5), // 5 requests per second
		ratelimit.WithBurst(10),            // Allow burst of 10
	))

	r.GET("/basic", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Rate limited to 5 req/s globally",
		})
	})
}

// Example 2: Per-IP rate limiting
func perIPExample(r *router.Router) {
	api := r.Group("/api")

	api.Use(ratelimit.New(
		ratelimit.WithRequestsPerSecond(10),
		ratelimit.WithBurst(20),
		ratelimit.WithKeyFunc(func(c *router.Context) string {
			return c.ClientIP() // Rate limit per client IP
		}),
	))

	api.GET("/public", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"message": "Rate limited per IP (10 req/s)",
			"ip":      c.ClientIP(),
		})
	})
}

// Example 3: API key-based rate limiting
func apiKeyExample(r *router.Router) {
	premium := r.Group("/api/premium")

	premium.Use(ratelimit.New(
		ratelimit.WithRequestsPerSecond(100),
		ratelimit.WithBurst(200),
		ratelimit.WithKeyFunc(func(c *router.Context) string {
			apiKey := c.Query("key")
			if apiKey == "" {
				return c.ClientIP() // Fall back to IP if no key
			}
			return "api_key:" + apiKey
		}),
	))

	premium.GET("", func(c *router.Context) {
		apiKey := c.Query("key")
		c.JSON(http.StatusOK, map[string]any{
			"message": "Premium API with higher limits",
			"key":     apiKey,
			"limit":   "100 req/s",
		})
	})
}

// Example 4: Tiered limits for different endpoints
func tieredLimitsExample(r *router.Router) {
	// Free tier: 1 req/sec (~60/min)
	free := r.Group("/free")
	free.Use(ratelimit.New(
		ratelimit.WithRequestsPerSecond(1), // Approximately 60 per minute
		ratelimit.WithBurst(10),
		ratelimit.WithKeyFunc(func(c *router.Context) string {
			return "free:" + c.ClientIP()
		}),
	))
	free.GET("/data", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"tier":  "free",
			"limit": "1 request per second (~60/min)",
		})
	})

	// Pro tier: High rate limit
	pro := r.Group("/pro")
	pro.Use(ratelimit.New(
		ratelimit.WithRequestsPerSecond(20), // 20 req/sec = ~1200 req/min
		ratelimit.WithBurst(100),
		ratelimit.WithKeyFunc(func(c *router.Context) string {
			return "pro:" + c.Query("token")
		}),
	))
	pro.GET("/data", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"tier":  "pro",
			"limit": "20 requests per second (~1200/min)",
		})
	})
}
