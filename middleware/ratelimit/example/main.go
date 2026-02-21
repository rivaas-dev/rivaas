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
	"log"
	"net/http"

	"rivaas.dev/router"
	"rivaas.dev/middleware/ratelimit"
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

	log.Println("Server starting on http://localhost:8080")
	log.Println("Endpoints: /basic /api/public /api/premium /free/data /pro/data")
	log.Fatal(http.ListenAndServe(":8080", r))
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
