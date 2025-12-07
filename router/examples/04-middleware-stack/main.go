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

// Package main demonstrates comprehensive middleware patterns including global, group-level,
// per-route, conditional, and custom middleware implementations.
package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"rivaas.dev/logging"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware/accesslog"
	"rivaas.dev/router/middleware/cors"
	"rivaas.dev/router/middleware/recovery"
	"rivaas.dev/router/middleware/timeout"
)

func main() {
	r := router.MustNew()

	// Set up logging for accesslog middleware
	logCfg := logging.MustNew(
		logging.WithConsoleHandler(),
		logging.WithDebugLevel(),
	)
	r.SetLogger(logCfg)

	// Global Middleware: applies to all routes
	r.Use(requestIDMiddleware())
	r.Use(accesslog.New())
	r.Use(recovery.New())

	// Conditional Middleware: based on path or conditions
	r.Use(func(c *router.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Header("X-API-Version", "v1")
			c.Header("X-Powered-By", "Rivaas Router")
		}
		c.Next()
	})

	// Group-Level Middleware
	api := r.Group("/api")
	api.Use(apiMiddleware())
	api.Use(cors.New(cors.WithAllowAllOrigins(true)))

	api.GET("/data", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message":    "API data with group middleware",
			"request_id": c.Request.Header.Get("X-Request-ID"),
		})
	})

	// Per-Route Middleware: passed as arguments
	r.GET("/protected", authMiddleware(), rateLimitMiddleware(), func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Protected resource accessed",
		})
	})

	// Nested Group Middleware: each level adds its own middleware
	admin := r.Group("/admin")
	admin.Use(authMiddleware())
	admin.Use(roleMiddleware("admin"))

	dashboard := admin.Group("/dashboard")
	dashboard.Use(auditMiddleware())

	dashboard.GET("/stats", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"stats":      "Dashboard statistics",
			"middleware": []string{"auth", "role:admin", "audit"},
		})
	})

	// Middleware with Skip Paths
	r.Use(accesslog.New(
		accesslog.WithExcludePaths("/health", "/metrics"),
	))

	r.GET("/health", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "healthy"})
	})

	// Middleware Order Demonstration
	ordered := r.Group("/ordered")
	ordered.Use(firstMiddleware())
	ordered.Use(secondMiddleware())
	ordered.Use(thirdMiddleware())

	ordered.GET("/demo", func(c *router.Context) {
		order := c.Request.Header.Get("X-Execution-Order")
		c.JSON(http.StatusOK, map[string]string{
			"message": "Check X-Execution-Order header",
			"order":   order,
		})
	})

	// Early Termination: abort middleware chain
	r.GET("/may-abort", func(c *router.Context) {
		authorized := c.Query("auth") == "true"
		if !authorized {
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Unauthorized - use ?auth=true",
			})
			c.Abort()
			return
		}
		c.JSON(http.StatusOK, map[string]string{"message": "Authorized"})
	})

	// Middleware with Configuration
	timeout := timeout.New(5 * time.Second)
	r.GET("/slow", timeout, func(c *router.Context) {
		time.Sleep(2 * time.Second)
		c.JSON(http.StatusOK, map[string]string{"message": "Completed"})
	})

	// Custom Middleware Examples

	// Simple middleware (no configuration)
	r.GET("/simple", requestIDMiddleware(), func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message":    "Simple middleware example",
			"request_id": c.Request.Header.Get("X-Request-ID"),
		})
	})

	// Middleware with configuration
	r.Use(loggerMiddleware(LoggerConfig{
		Format:    "json",
		SkipPaths: []string{"/health"},
	}))

	// Conditional middleware factory
	r.Use(conditionalMiddleware(conditionalConfig{
		OnlyPaths: []string{"/api"},
		Header:    "X-API-Enabled",
		Value:     "true",
	}))

	r.GET("/api/data", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"data":   "API data",
			"header": c.Request.Header.Get("X-API-Enabled"),
		})
	})

	// Timing middleware
	r.Use(timingMiddleware())
	r.GET("/timed", func(c *router.Context) {
		time.Sleep(100 * time.Millisecond)
		c.JSON(http.StatusOK, map[string]string{
			"message":       "Check X-Response-Time header",
			"response_time": c.Request.Header.Get("X-Response-Time"),
		})
	})

	// Authentication middleware factory
	authRequired := createAuthMiddleware(authConfig{
		Required: true,
		Header:   "Authorization",
	})

	protected := r.Group("/protected")
	protected.Use(authRequired)
	protected.GET("/secret", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "This is a protected resource",
			"user":    c.Request.Header.Get("X-User-ID"),
		})
	})

	// Rate limiting middleware
	rateLimiter := createRateLimitMiddleware(rateLimitConfig{
		MaxRequests: 5,
		Window:      1 * time.Minute,
	})

	r.GET("/rate-limited", rateLimiter, func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Rate limited endpoint",
		})
	})

	// Request validation middleware
	validateJSON := validateRequestMiddleware(validateConfig{
		RequiredContentType: "application/json",
		RequiredHeaders:     []string{"X-Client-Version"},
	})

	r.POST("/validated", validateJSON, func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Request validated successfully",
		})
	})

	// Middleware with state (using closure)
	counter := requestCounterMiddleware()
	r.GET("/counter", counter, func(c *router.Context) {
		count := c.Request.Header.Get("X-Request-Count")
		c.JSON(http.StatusOK, map[string]string{
			"message": fmt.Sprintf("Request #%s", count),
		})
	})

	// Combining Multiple Patterns
	complexGroup := r.Group("/complex")
	complexGroup.Use(
		requestIDMiddleware(),
		timingMiddleware(),
		authMiddleware(),
	)
	complexGroup.Use(func(c *router.Context) {
		c.Header("X-Complex-Route", "true")
		c.Next()
	})

	complexGroup.GET("/endpoint", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]string{
			"message": "Complex route with multiple middleware",
		})
	})

	// Documentation endpoint
	r.GET("/", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"message": "Middleware Stack Demo",
			"patterns": []map[string]string{
				{"pattern": "Global middleware", "example": "Applies to all routes"},
				{"pattern": "Conditional middleware", "example": "Based on path or conditions"},
				{"pattern": "Group middleware", "example": "/api/* has API middleware"},
				{"pattern": "Per-route middleware", "example": "/protected has auth + rate limit"},
				{"pattern": "Nested groups", "example": "/admin/dashboard/* has 3 layers"},
				{"pattern": "Skip paths", "example": "/health skips logging"},
				{"pattern": "Middleware order", "example": "/ordered/demo shows execution order"},
				{"pattern": "Early termination", "example": "/may-abort can abort chain"},
				{"pattern": "With configuration", "example": "/slow has 5s timeout"},
				{"pattern": "Custom middleware", "example": "Rate limiting, validation, etc."},
				{"pattern": "Complex combining", "example": "/complex/endpoint combines patterns"},
			},
			"endpoints": map[string]string{
				"GET /api/data":              "Group middleware",
				"GET /protected":             "Per-route middleware",
				"GET /admin/dashboard/stats": "Nested group middleware",
				"GET /health":                "Skipped from logging",
				"GET /ordered/demo":          "Shows middleware order",
				"GET /may-abort?auth=true":   "Conditional abort",
				"GET /slow":                  "With timeout",
				"GET /simple":                "Simple middleware",
				"GET /timed":                 "Timing middleware",
				"GET /protected/secret":      "Auth middleware (use Authorization header)",
				"GET /rate-limited":          "Rate limiting (try 6+ requests)",
				"POST /validated":            "Request validation",
				"GET /counter":               "Stateful middleware",
				"GET /complex/endpoint":      "Multiple patterns combined",
			},
		})
	})

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	port := ":8080"
	logger.Info("🚀 Server starting on http://localhost" + port)
	logger.Print("")
	logger.Print("📋 Example commands:")
	logger.Print("  curl http://localhost:8080/")
	logger.Print("  curl http://localhost:8080/api/data")
	logger.Print("  curl http://localhost:8080/protected")
	logger.Print("  curl http://localhost:8080/admin/dashboard/stats")
	logger.Print("  curl http://localhost:8080/ordered/demo")
	logger.Print("  curl 'http://localhost:8080/may-abort?auth=true'")
	logger.Print("  curl -H 'Authorization: Bearer token' http://localhost:8080/protected/secret")
	logger.Print("  curl http://localhost:8080/rate-limited  # Try 6+ requests")
	logger.Print("")

	logger.Fatal(http.ListenAndServe(port, r))
}

// Common Middleware Implementations

// requestIDMiddleware generates or propagates request ID for tracing
func requestIDMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		requestID := c.Request.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		c.Request.Header.Set("X-Request-ID", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// apiMiddleware adds API-specific headers
func apiMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		c.Header("X-API-Middleware", "enabled")
		c.Next()
	}
}

// authMiddleware validates Authorization header and sets user context
func authMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		token := c.Request.Header.Get("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Missing authorization header",
			})
			c.Abort()
			return
		}
		// Simulate token validation
		if token == "Bearer token123" || token == "Bearer admin-token" {
			c.Request.Header.Set("X-User-ID", "123")
			if token == "Bearer admin-token" {
				c.Request.Header.Set("X-User-Role", "admin")
			}
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Invalid token",
			})
			c.Abort()
		}
	}
}

// rateLimitMiddleware is a simple rate limit example (always allows)
func rateLimitMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		c.Header("X-RateLimit-Remaining", "99")
		c.Next()
	}
}

// roleMiddleware enforces role-based access control
func roleMiddleware(requiredRole string) router.HandlerFunc {
	return func(c *router.Context) {
		role := c.Request.Header.Get("X-User-Role")
		if role != requiredRole {
			c.JSON(http.StatusForbidden, map[string]string{
				"error": fmt.Sprintf("%s access required", requiredRole),
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// auditMiddleware marks requests for audit logging
func auditMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		c.Header("X-Audit-Enabled", "true")
		c.Next()
	}
}

// loggingMiddlewareWithSkip logs requests except for specified paths
func loggingMiddlewareWithSkip(skipPaths []string) router.HandlerFunc {
	return func(c *router.Context) {
		path := c.Request.URL.Path
		for _, skip := range skipPaths {
			if path == skip {
				c.Next()
				return
			}
		}
		log.Info("Request", "method", c.Request.Method, "path", path)
		c.Next()
	}
}

// firstMiddleware, secondMiddleware, thirdMiddleware demonstrate execution order
func firstMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		order := c.Request.Header.Get("X-Execution-Order")
		c.Request.Header.Set("X-Execution-Order", order+"1->")
		c.Next()
	}
}

func secondMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		order := c.Request.Header.Get("X-Execution-Order")
		c.Request.Header.Set("X-Execution-Order", order+"2->")
		c.Next()
	}
}

func thirdMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		order := c.Request.Header.Get("X-Execution-Order")
		c.Request.Header.Set("X-Execution-Order", order+"3->")
		c.Next()
	}
}

// timingMiddleware measures and reports request processing time
func timingMiddleware() router.HandlerFunc {
	return func(c *router.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		c.Header("X-Response-Time", duration.String())
	}
}

// Custom Middleware with Configuration

// LoggerConfig holds configuration for the logger middleware.
type LoggerConfig struct {
	Format    string
	SkipPaths []string
}

// loggerMiddleware logs requests with configurable format and path skipping
func loggerMiddleware(config LoggerConfig) router.HandlerFunc {
	return func(c *router.Context) {
		path := c.Request.URL.Path

		// Skip logging for specified paths
		for _, skip := range config.SkipPaths {
			if path == skip {
				c.Next()
				return
			}
		}

		start := time.Now()
		c.Next()
		duration := time.Since(start)

		log.Info("Request", "method", c.Request.Method, "path", path, "duration", duration)
	}
}

type conditionalConfig struct {
	OnlyPaths []string
	Header    string
	Value     string
}

// conditionalMiddleware applies middleware only to specified paths
func conditionalMiddleware(config conditionalConfig) router.HandlerFunc {
	return func(c *router.Context) {
		path := c.Request.URL.Path

		for _, allowedPath := range config.OnlyPaths {
			if strings.HasPrefix(path, allowedPath) {
				c.Request.Header.Set(config.Header, config.Value)
				c.Header(config.Header, config.Value)
				break
			}
		}

		c.Next()
	}
}

type authConfig struct {
	Required bool
	Header   string
}

// createAuthMiddleware returns a configurable authentication middleware factory
func createAuthMiddleware(config authConfig) router.HandlerFunc {
	return func(c *router.Context) {
		if !config.Required {
			c.Next()
			return
		}

		token := c.Request.Header.Get(config.Header)
		if token == "" {
			c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Authorization required",
			})
			c.Abort()
			return
		}

		c.Request.Header.Set("X-User-ID", "user-123")
		c.Next()
	}
}

type rateLimitConfig struct {
	MaxRequests int
	Window      time.Duration
}

var requestCounts = make(map[string][]time.Time)

// createRateLimitMiddleware returns a rate-limiting middleware with sliding window
func createRateLimitMiddleware(config rateLimitConfig) router.HandlerFunc {
	return func(c *router.Context) {
		ip := c.ClientIP()
		now := time.Now()

		// Clean up old requests outside the time window
		cutoff := now.Add(-config.Window)
		requests := requestCounts[ip]
		validRequests := []time.Time{}
		for _, t := range requests {
			if t.After(cutoff) {
				validRequests = append(validRequests, t)
			}
		}

		if len(validRequests) >= config.MaxRequests {
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", config.MaxRequests))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("Retry-After", fmt.Sprintf("%.0f", config.Window.Seconds()))
			c.JSON(http.StatusTooManyRequests, map[string]string{
				"error": "Rate limit exceeded",
			})
			c.Abort()
			return
		}

		validRequests = append(validRequests, now)
		requestCounts[ip] = validRequests

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", config.MaxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", config.MaxRequests-len(validRequests)))

		c.Next()
	}
}

type validateConfig struct {
	RequiredContentType string
	RequiredHeaders     []string
}

// validateRequestMiddleware validates request content type and required headers
func validateRequestMiddleware(config validateConfig) router.HandlerFunc {
	return func(c *router.Context) {
		if config.RequiredContentType != "" {
			contentType := c.Request.Header.Get("Content-Type")
			if !strings.Contains(contentType, config.RequiredContentType) {
				c.JSON(http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("Content-Type must be %s", config.RequiredContentType),
				})
				c.Abort()
				return
			}
		}

		for _, header := range config.RequiredHeaders {
			if c.Request.Header.Get(header) == "" {
				c.JSON(http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("Missing required header: %s", header),
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// requestCounterMiddleware uses closure to maintain state across requests
func requestCounterMiddleware() router.HandlerFunc {
	var count int
	return func(c *router.Context) {
		count++
		c.Request.Header.Set("X-Request-Count", fmt.Sprintf("%d", count))
		c.Next()
	}
}
