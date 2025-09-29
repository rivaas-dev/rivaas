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

// Package main provides examples of using the Compression middleware.
package main

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"rivaas.dev/router"
	"rivaas.dev/router/middleware/compression"
)

func main() {
	r := router.MustNew()

	// Example 1: Basic compression with defaults
	basicExample(r)

	// Example 2: Custom compression level
	customLevelExample(r)

	// Example 3: Exclude certain paths and file types
	excludeExample(r)

	// Example 4: Production-ready configuration
	productionExample(r)

	// Example 5: Compression ratio demonstration
	compressionRatioExample(r)

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	logger.Info("🚀 Server starting on http://localhost:8080")
	logger.Print("")
	logger.Print("📝 Available endpoints:")
	logger.Print("  GET /basic        - Basic compression")
	logger.Print("  GET /fast         - Fast compression (best speed)")
	logger.Print("  GET /best         - Best compression (smallest size)")
	logger.Print("  GET /large        - Large response (shows compression benefits)")
	logger.Print("  GET /image.jpg    - Excluded by extension")
	logger.Print("  GET /metrics      - Excluded path")
	logger.Print("  GET /ratio        - Compression ratio demo")
	logger.Print("  GET /api/users    - Production configuration")
	logger.Print("")
	logger.Print("📋 Example commands:")
	logger.Print("  curl -H 'Accept-Encoding: gzip' http://localhost:8080/basic -i")
	logger.Print("  curl -H 'Accept-Encoding: gzip' http://localhost:8080/ratio --compressed -w '\\nSize: %{size_download} bytes\\n'")
	logger.Print("")
	logger.Print("💡 Tip: Use --compressed flag to auto-decompress responses")
	logger.Print("")

	logger.Fatal(http.ListenAndServe(":8080", r))
}

// Example 1: Basic compression with defaults
func basicExample(r *router.Router) {
	basic := r.Group("/basic")

	// Use default compression settings
	basic.Use(compression.New())

	basic.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"message": "This response will be compressed with default settings",
			"info":    "Default compression level is -1 (usually level 6)",
			"minSize": "Minimum 1KB to compress",
		})
	})
}

// Example 2: Custom compression levels
func customLevelExample(r *router.Router) {
	// Best speed (level 1)
	fast := r.Group("/fast")
	fast.Use(compression.New(
		compression.WithGzipLevel(gzip.BestSpeed),
	))
	fast.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"message": "Compressed with BestSpeed (fastest, larger size)",
			"data":    strings.Repeat("sample data ", 100),
		})
	})

	// Best compression (level 9)
	best := r.Group("/best")
	best.Use(compression.New(
		compression.WithGzipLevel(gzip.BestCompression),
	))
	best.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"message": "Compressed with BestCompression (slowest, smallest size)",
			"data":    strings.Repeat("sample data ", 100),
		})
	})

	// Large response to see compression benefits
	r.GET("/large", func(c *router.Context) {
		// Generate a large JSON response
		data := make([]map[string]string, 1000)
		for i := range data {
			data[i] = map[string]string{
				"id":          fmt.Sprintf("item-%d", i),
				"name":        fmt.Sprintf("Item %d", i),
				"description": "This is a sample item description that will be repeated many times.",
			}
		}
		c.JSON(http.StatusOK, data)
	})
}

// Example 3: Exclude certain paths and file types
func excludeExample(r *router.Router) {
	r.Use(compression.New(
		// Don't compress already compressed formats
		compression.WithExcludeExtensions(".jpg", ".png", ".gif", ".zip", ".gz"),

		// Don't compress metrics endpoint (often scraped by tools)
		compression.WithExcludePaths("/metrics"),

		// Don't compress certain content types
		compression.WithExcludeContentTypes("image/jpeg", "image/png", "application/zip"),

		// Only compress responses larger than 2KB
		compression.WithMinSize(2048),
	))

	r.GET("/image.jpg", func(c *router.Context) {
		c.Response.Header().Set("Content-Type", "image/jpeg")
		c.String(http.StatusOK, "This simulates image data - won't be compressed")
	})

	r.GET("/metrics", func(c *router.Context) {
		c.String(http.StatusOK, "# Metrics endpoint - not compressed")
	})
}

// Example 4: Production-ready configuration
func productionExample(r *router.Router) {
	api := r.Group("/api")

	// Production compression settings
	api.Use(compression.New(
		// Use default compression (good balance of speed vs size)
		compression.WithGzipLevel(gzip.DefaultCompression),

		// Only compress responses >= 1KB
		compression.WithMinSize(1024),

		// Exclude pre-compressed formats
		compression.WithExcludeExtensions(
			".jpg", ".jpeg", ".png", ".gif", ".webp", // Images
			".zip", ".gz", ".br", // Archives
			".mp4", ".avi", ".mov", // Videos
			".mp3", ".wav", ".ogg", // Audio
			".woff", ".woff2", // Fonts
		),

		// Exclude content types that don't benefit from compression
		compression.WithExcludeContentTypes(
			"image/",
			"video/",
			"audio/",
			"application/zip",
			"application/x-gzip",
		),

		// Exclude monitoring endpoints
		compression.WithExcludePaths(
			"/health",
			"/metrics",
			"/readiness",
			"/liveness",
		),
	))

	api.GET("/users", func(c *router.Context) {
		// Simulate a list of users
		users := make([]map[string]any, 50)
		for i := range users {
			users[i] = map[string]any{
				"id":    i,
				"name":  fmt.Sprintf("User %d", i),
				"email": fmt.Sprintf("user%d@example.com", i),
			}
		}
		c.JSON(http.StatusOK, users)
	})
}

// Example 5: Compression ratio demonstration
func compressionRatioExample(r *router.Router) {
	r.Use(compression.New())

	r.GET("/ratio", func(c *router.Context) {
		// Large repetitive data compresses very well
		data := map[string]any{
			"message": "Compression works best with repetitive data",
			"items":   strings.Repeat("repeated data ", 500),
			"status":  "success",
		}
		c.JSON(http.StatusOK, data)
	})
}
