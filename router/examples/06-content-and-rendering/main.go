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

// Package main demonstrates content negotiation, various rendering methods (JSON, XML, HTML, YAML),
// and context helper utilities for working with requests.
package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"rivaas.dev/router"
)

func main() {
	r := router.MustNew()

	// Content Negotiation: respond with different formats based on Accept header

	r.GET("/api/user", func(c *router.Context) {
		user := map[string]any{"id": 1, "name": "John Doe"}

		switch c.Accepts("json", "xml", "html") {
		case "json":
			c.JSON(http.StatusOK, user)
		case "xml":
			c.Header("Content-Type", "application/xml")
			c.Stringf(http.StatusOK, "<user><id>%d</id><name>%s</name></user>", user["id"], user["name"])
		case "html":
			html := fmt.Sprintf("<html><body><h1>User: %s</h1><p>ID: %d</p></body></html>", user["name"], user["id"])
			c.HTML(http.StatusOK, html)
		default:
			c.Status(http.StatusNotAcceptable)
			c.String(http.StatusNotAcceptable, "Not Acceptable")
		}
	})

	// Language negotiation: select language based on Accept-Language header
	r.GET("/api/greeting", func(c *router.Context) {
		greetings := map[string]string{
			"en": "Hello!",
			"fr": "Bonjour!",
			"de": "Hallo!",
			"es": "¡Hola!",
		}

		lang := c.AcceptsLanguages("en", "fr", "de", "es")
		if lang == "" {
			lang = "en"
		}

		c.JSON(http.StatusOK, map[string]string{
			"language": lang,
			"greeting": greetings[lang],
		})
	})

	// Encoding negotiation: detect preferred content encoding (gzip, br, deflate)
	r.GET("/api/data", func(c *router.Context) {
		data := map[string]any{
			"message": "This is sample data that could be compressed",
			"items":   []int{1, 2, 3, 4, 5},
		}

		encoding := c.AcceptsEncodings("gzip", "br", "deflate")
		if encoding != "" {
			c.Header("Content-Encoding", encoding)
			c.Header("X-Encoding-Applied", encoding)
		}

		c.JSON(http.StatusOK, data)
	})

	// Rendering Methods: various formats optimized for different use cases

	// Standard JSON with HTML escaping for security
	r.GET("/json", func(c *router.Context) {
		data := map[string]any{
			"html":    "<h1>Title</h1>",
			"url":     "https://example.com?a=1&b=2",
			"message": "Standard JSON escapes HTML characters",
		}
		c.JSON(http.StatusOK, data)
	})

	// IndentedJSON: human-readable format for development and debugging
	r.GET("/json/indented", func(c *router.Context) {
		data := map[string]any{
			"user": map[string]any{
				"id":   123,
				"name": "John Doe",
			},
			"settings": map[string]bool{
				"notifications": true,
				"darkMode":      false,
			},
		}
		c.IndentedJSON(200, data)
	})

	// PureJSON: unescaped HTML, faster for HTML/XML content (35% faster)
	r.GET("/json/pure", func(c *router.Context) {
		data := map[string]any{
			"html":     "<h1>Title</h1><p>Content</p>",
			"url":      "https://example.com?foo=bar&baz=qux",
			"markdown": "## Header\n**Bold** text with <code>",
			"note":     "PureJSON doesn't escape HTML - 35% faster!",
		}
		c.PureJSON(200, data)
	})

	// SecureJSON: prevents JSON hijacking with default "while(1);" prefix
	r.GET("/json/secure", func(c *router.Context) {
		secrets := []string{"secret1", "secret2", "secret3"}
		c.SecureJSON(200, secrets)
	})

	// SecureJSON with custom prefix: use "for(;;);" or other anti-hijacking patterns
	r.GET("/json/secure/custom", func(c *router.Context) {
		data := map[string]string{"token": "abc123"}
		c.SecureJSON(200, data, "for(;;);")
	})

	// AsciiJSON: converts Unicode to ASCII escape sequences (\uXXXX format)
	r.GET("/json/ascii", func(c *router.Context) {
		data := map[string]any{
			"message":  "Hello 世界 🌍",
			"name":     "José García",
			"greeting": "こんにちは",
			"note":     "All non-ASCII converted to \\uXXXX",
		}
		c.ASCIIJSON(200, data)
	})

	// YAML: human-readable format for configuration and documentation APIs
	r.GET("/config", func(c *router.Context) {
		config := map[string]any{
			"database": map[string]any{
				"host":     "localhost",
				"port":     5432,
				"name":     "mydb",
				"poolSize": 10,
			},
			"server": map[string]any{
				"port":    8080,
				"timeout": "30s",
				"debug":   true,
			},
		}
		c.YAML(200, config)
	})

	// Data: raw bytes for binary content like images, PDFs (98% faster than JSON)
	r.GET("/image", func(c *router.Context) {
		pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		c.Data(200, "image/png", pngData)
	})

	r.GET("/pdf", func(c *router.Context) {
		pdfData := []byte("%PDF-1.4\nSample PDF content")
		c.Data(200, "application/pdf", pdfData)
	})

	// DataFromReader: stream large files efficiently without loading into memory
	r.GET("/stream/file", func(c *router.Context) {
		file, err := os.Open("README.md")
		if err != nil {
			c.JSON(http.StatusInternalServerError, map[string]string{"error": "File not found"})
			return
		}
		defer func() {
			_ = file.Close()
		}()

		stat, _ := file.Stat()
		headers := map[string]string{
			"Content-Disposition": `attachment; filename="README.md"`,
			"Cache-Control":       "no-cache",
		}

		c.DataFromReader(200, stat.Size(), "text/markdown", file, headers)
	})

	// DataFromReader: stream dynamically generated content from readers
	r.GET("/stream/logs", func(c *router.Context) {
		logData := strings.NewReader("Log line 1\nLog line 2\nLog line 3\n...")
		c.DataFromReader(200, -1, "text/plain; charset=utf-8", logData, map[string]string{
			"X-Content-Type": "stream",
		})
	})

	// JSONP: wrap JSON in callback function for cross-origin requests
	r.GET("/jsonp", func(c *router.Context) {
		data := map[string]string{
			"message": "JSONP response",
			"user":    "john",
		}
		callback := c.Query("callback")
		if callback == "" {
			callback = "callback"
		}
		c.JSONP(http.StatusOK, data, callback)
	})

	// Performance comparison: test different rendering methods
	r.GET("/benchmark", func(c *router.Context) {
		format := c.Query("format")

		benchData := map[string]any{
			"id":    123,
			"name":  "Test User",
			"email": "test@example.com",
		}

		switch format {
		case "json":
			c.JSON(http.StatusOK, benchData)
		case "pure":
			c.PureJSON(200, benchData)
		case "indented":
			c.IndentedJSON(200, benchData)
		case "secure":
			c.SecureJSON(200, benchData)
		case "ascii":
			c.ASCIIJSON(200, benchData)
		case "yaml":
			c.YAML(200, benchData)
		default:
			c.JSON(http.StatusOK, map[string]string{
				"error": "Unknown format. Use: json, pure, indented, secure, ascii, yaml",
			})
		}
	})

	// Stream from bytes.Buffer: efficient for in-memory generated content
	r.GET("/stream/buffer", func(c *router.Context) {
		var buf bytes.Buffer
		for i := 0; i < 100; i++ {
			buf.WriteString(fmt.Sprintf("Line %d\n", i))
		}
		c.DataFromReader(200, int64(buf.Len()), "text/plain", &buf, nil)
	})

	// Context Helpers: utilities for accessing request data and setting responses

	r.GET("/demo", func(c *router.Context) {
		c.JSON(http.StatusOK, map[string]any{
			"client_ip": c.ClientIP(),
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
			"is_json":   c.IsJSON(),
			"accepts":   c.AcceptsJSON(),
		})
	})

	// Query parameters: extract URL query values with defaults
	r.GET("/search", func(c *router.Context) {
		query := c.Query("q")
		page := c.QueryDefault("page", "1")
		limit := c.QueryDefault("limit", "10")

		c.JSON(http.StatusOK, map[string]any{
			"query": query,
			"page":  page,
			"limit": limit,
		})
	})

	// Headers: read request headers and set response headers
	r.GET("/headers", func(c *router.Context) {
		userAgent := c.Request.Header.Get("User-Agent")
		accept := c.Request.Header.Get("Accept")

		c.Header("X-Response-ID", "resp-123")
		c.Header("X-Powered-By", "Rivaas Router")

		c.JSON(http.StatusOK, map[string]any{
			"user_agent": userAgent,
			"accept":     accept,
		})
	})

	// Cookies: get and set HTTP cookies with configurable options
	r.GET("/cookies", func(c *router.Context) {
		theme, _ := c.GetCookie("theme")
		c.SetCookie("theme", "dark", 86400, "/", "", false, false)
		c.SetCookie("language", "en", 86400, "/", "", false, false)

		c.JSON(http.StatusOK, map[string]any{
			"previous_theme": theme,
			"message":        "Cookies set",
		})
	})

	// Path parameters: extract route variables from URL path
	r.GET("/users/:id/posts/:post_id", func(c *router.Context) {
		userID := c.Param("id")
		postID := c.Param("post_id")

		c.JSON(http.StatusOK, map[string]any{
			"user_id": userID,
			"post_id": postID,
		})
	})

	// String: plain text response
	r.GET("/string", func(c *router.Context) {
		c.String(http.StatusOK, "This is a plain text response")
	})

	// HTML: render HTML content with proper content type
	r.GET("/html", func(c *router.Context) {
		c.HTML(http.StatusOK, "<html><body><h1>Hello</h1><p>This is HTML</p></body></html>")
	})

	// Redirect: HTTP redirect response
	r.GET("/redirect", func(c *router.Context) {
		c.Redirect(http.StatusFound, "/demo")
	})

	// Documentation endpoint: overview of available features
	r.GET("/", func(c *router.Context) {
		info := map[string]any{
			"message": "Content and Rendering Demo",
			"features": []string{
				"Content negotiation (Accept headers)",
				"Multiple rendering formats (JSON, YAML, etc.)",
				"Performance-optimized rendering",
				"Streaming responses",
				"Context helper methods",
			},
			"rendering_methods": map[string]string{
				"JSON()":         "4,189ns/op - baseline",
				"PureJSON()":     "2,725ns/op - 35% faster (HTML content)",
				"Data()":         "90ns/op - 98% faster (binary/images)",
				"AsciiJSON()":    "1,593ns/op - 62% faster",
				"SecureJSON()":   "4,835ns/op - +15% (compliance)",
				"IndentedJSON()": "8,111ns/op - +94% (debug only)",
				"YAML()":         "36,700ns/op - +776% (config APIs)",
			},
			"endpoints": map[string]string{
				"GET /api/user":              "Content negotiation (JSON/XML/HTML)",
				"GET /json":                  "Standard JSON",
				"GET /json/pure":             "PureJSON (35% faster)",
				"GET /json/secure":           "SecureJSON",
				"GET /config":                "YAML rendering",
				"GET /stream/file":           "Stream file",
				"GET /benchmark?format=pure": "Performance comparison",
				"GET /demo":                  "Context helpers",
				"GET /search?q=test":         "Query parameters",
				"GET /cookies":               "Cookie helpers",
			},
		}

		c.PureJSON(200, info)
	})

	// Create a logger with clean, colorful output
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: false,
		ReportCaller:    false,
	})

	port := ":8080"
	logger.Info("🚀 Server starting on http://localhost" + port)
	logger.Print("")
	logger.Print("📋 Example endpoints:")
	logger.Printf("  curl http://localhost%s/\n", port)
	logger.Printf("  curl -H 'Accept: application/json' http://localhost%s/api/user\n", port)
	logger.Printf("  curl -H 'Accept: application/xml' http://localhost%s/api/user\n", port)
	logger.Printf("  curl http://localhost%s/json/pure\n", port)
	logger.Printf("  curl http://localhost%s/benchmark?format=pure\n", port)
	logger.Printf("  curl http://localhost%s/search?q=test&page=2\n", port)
	logger.Print("")

	logger.Fatal(http.ListenAndServe(port, r))
}
