package main

import (
	"compress/gzip"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/rivaas-dev/rivaas/router"
	"github.com/rivaas-dev/rivaas/router/middleware"
)

func main() {
	r := router.New()

	// Example 1: Basic compression with defaults
	basicExample(r)

	// Example 2: Custom compression level
	customLevelExample(r)

	// Example 3: Exclude certain paths and file types
	excludeExample(r)

	// Example 4: Production-ready configuration
	productionExample(r)

	fmt.Println("Server starting on :8080")
	fmt.Println("Try these endpoints:")
	fmt.Println("  - GET /basic - Basic compression")
	fmt.Println("  - GET /fast - Fast compression (best speed)")
	fmt.Println("  - GET /best - Best compression (smallest size)")
	fmt.Println("  - GET /large - Large response (shows compression benefits)")
	fmt.Println("  - GET /image.jpg - Excluded by extension")
	fmt.Println("  - GET /metrics - Excluded path")
	fmt.Println("")
	fmt.Println("Test with: curl -H 'Accept-Encoding: gzip' http://localhost:8080/basic -i")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}

// Example 1: Basic compression with defaults
func basicExample(r *router.Router) {
	basic := r.Group("/basic")

	// Use default compression settings
	basic.Use(middleware.Compression())

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
	fast.Use(middleware.Compression(
		middleware.WithCompressionLevel(gzip.BestSpeed),
	))
	fast.GET("", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"message": "Compressed with BestSpeed (fastest, larger size)",
			"data":    strings.Repeat("sample data ", 100),
		})
	})

	// Best compression (level 9)
	best := r.Group("/best")
	best.Use(middleware.Compression(
		middleware.WithCompressionLevel(gzip.BestCompression),
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
	r.Use(middleware.Compression(
		// Don't compress already compressed formats
		middleware.WithExcludeExtensions([]string{".jpg", ".png", ".gif", ".zip", ".gz"}),

		// Don't compress metrics endpoint (often scraped by tools)
		middleware.WithExcludePaths([]string{"/metrics"}),

		// Don't compress certain content types
		middleware.WithExcludeContentTypes([]string{"image/jpeg", "image/png", "application/zip"}),

		// Only compress responses larger than 2KB
		middleware.WithMinSize(2048),
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
	api.Use(middleware.Compression(
		// Use default compression (good balance of speed vs size)
		middleware.WithCompressionLevel(gzip.DefaultCompression),

		// Only compress responses >= 1KB
		middleware.WithMinSize(1024),

		// Exclude pre-compressed formats
		middleware.WithExcludeExtensions([]string{
			".jpg", ".jpeg", ".png", ".gif", ".webp", // Images
			".zip", ".gz", ".br", // Archives
			".mp4", ".avi", ".mov", // Videos
			".mp3", ".wav", ".ogg", // Audio
			".woff", ".woff2", // Fonts
		}),

		// Exclude content types that don't benefit from compression
		middleware.WithExcludeContentTypes([]string{
			"image/",
			"video/",
			"audio/",
			"application/zip",
			"application/x-gzip",
		}),

		// Exclude monitoring endpoints
		middleware.WithExcludePaths([]string{
			"/health",
			"/metrics",
			"/readiness",
			"/liveness",
		}),
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

// Example showing compression ratio
func compressionRatioExample() {
	r := router.New()
	r.Use(middleware.Compression())

	r.GET("/ratio", func(c *router.Context) {
		// Large repetitive data compresses very well
		data := map[string]any{
			"message": "Compression works best with repetitive data",
			"items":   strings.Repeat("repeated data ", 500),
			"status":  "success",
		}
		c.JSON(http.StatusOK, data)
	})

	fmt.Println("Try: curl -H 'Accept-Encoding: gzip' http://localhost:8080/ratio --compressed -w '\\nSize: %{size_download} bytes\\n'")
}
