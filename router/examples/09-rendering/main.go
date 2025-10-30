package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"rivaas.dev/router"
)

func main() {
	r := router.New()

	// 1. Standard JSON (HTML-escaped)
	r.GET("/json", func(c *router.Context) {
		data := map[string]any{
			"html":    "<h1>Title</h1>",
			"url":     "https://example.com?a=1&b=2",
			"message": "Standard JSON escapes HTML characters",
		}
		c.JSON(200, data)
		// Output: {"html":"\u003ch1\u003e...","url":"https://example.com?a=1\u0026b=2"}
	})

	// 2. IndentedJSON - Pretty-printed for debugging
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
		// Output:
		// {
		//   "user": {
		//     "id": 123,
		//     "name": "John Doe"
		//   },
		//   ...
		// }
	})

	// 3. PureJSON - Unescaped HTML (35% faster!)
	r.GET("/json/pure", func(c *router.Context) {
		data := map[string]any{
			"html":     "<h1>Title</h1><p>Content</p>",
			"url":      "https://example.com?foo=bar&baz=qux",
			"markdown": "## Header\n**Bold** text with <code>",
			"note":     "PureJSON doesn't escape HTML - 35% faster!",
		}
		c.PureJSON(200, data)
		// Output: {"html":"<h1>Title</h1>","url":"https://example.com?foo=bar&baz=qux"}
	})

	// 4. SecureJSON - Anti-hijacking prefix for compliance
	r.GET("/json/secure", func(c *router.Context) {
		// Critical for protecting sensitive array data from old browser vulnerabilities
		secrets := []string{"secret1", "secret2", "secret3"}
		c.SecureJSON(200, secrets)
		// Output: while(1);["secret1","secret2","secret3"]
		// Client must strip "while(1);" prefix before parsing
	})

	// 5. SecureJSON with custom prefix
	r.GET("/json/secure/custom", func(c *router.Context) {
		data := map[string]string{"token": "abc123"}
		c.SecureJSON(200, data, "for(;;);")
		// Output: for(;;);{"token":"abc123"}
	})

	// 6. AsciiJSON - Pure ASCII with Unicode escaping
	r.GET("/json/ascii", func(c *router.Context) {
		data := map[string]any{
			"message":  "Hello 世界 🌍",
			"name":     "José García",
			"greeting": "こんにちは",
			"note":     "All non-ASCII converted to \\uXXXX",
		}
		c.AsciiJSON(200, data)
		// Output: {"message":"Hello \u4e16\u754c \ud83c\udf0d","name":"Jos\u00e9",...}
	})

	// 7. YAML - Config-style APIs
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
			"logging": map[string]string{
				"level":  "info",
				"format": "json",
			},
		}
		c.YAML(200, config)
		// Output:
		// database:
		//   host: localhost
		//   port: 5432
		//   ...
	})

	// 8. Data - Custom content types (98% faster than JSON!)
	r.GET("/image", func(c *router.Context) {
		// Simulated PNG header
		pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
		c.Data(200, "image/png", pngData)
	})

	r.GET("/pdf", func(c *router.Context) {
		pdfData := []byte("%PDF-1.4\nSample PDF content")
		c.Data(200, "application/pdf", pdfData)
	})

	// 9. DataFromReader - Zero-copy streaming for large files
	r.GET("/stream/file", func(c *router.Context) {
		file, err := os.Open("README.md")
		if err != nil {
			c.JSON(500, map[string]string{"error": "File not found"})
			return
		}
		defer file.Close()

		stat, _ := file.Stat()
		headers := map[string]string{
			"Content-Disposition": `attachment; filename="README.md"`,
			"Cache-Control":       "no-cache",
		}

		c.DataFromReader(200, stat.Size(), "text/markdown", file, headers)
		// Streams file without buffering entire content in memory
	})

	// 10. DataFromReader - Stream generated data
	r.GET("/stream/logs", func(c *router.Context) {
		// Simulate streaming logs
		logData := strings.NewReader("Log line 1\nLog line 2\nLog line 3\n...")

		c.DataFromReader(200, -1, "text/plain; charset=utf-8", logData, map[string]string{
			"X-Content-Type": "stream",
		})
	})

	// 11. JSONP - Callback wrapper for cross-domain requests
	r.GET("/jsonp", func(c *router.Context) {
		data := map[string]string{
			"message": "JSONP response",
			"user":    "john",
		}
		callback := c.Query("callback")
		if callback == "" {
			callback = "callback"
		}
		c.JSONP(200, data, callback)
		// Output: callback({"message":"JSONP response","user":"john"})
	})

	// 12. Performance comparison endpoint
	r.GET("/benchmark", func(c *router.Context) {
		format := c.Query("format")

		benchData := map[string]any{
			"id":    123,
			"name":  "Test User",
			"email": "test@example.com",
		}

		switch format {
		case "json":
			c.JSON(200, benchData)
		case "pure":
			c.PureJSON(200, benchData) // 35% faster
		case "indented":
			c.IndentedJSON(200, benchData)
		case "secure":
			c.SecureJSON(200, benchData)
		case "ascii":
			c.AsciiJSON(200, benchData)
		case "yaml":
			c.YAML(200, benchData)
		default:
			c.JSON(200, map[string]string{
				"error": "Unknown format. Use: json, pure, indented, secure, ascii, yaml",
			})
		}
	})

	// 13. Binary data example
	r.GET("/binary", func(c *router.Context) {
		// Generate some binary data
		binaryData := make([]byte, 1024)
		for i := range binaryData {
			binaryData[i] = byte(i % 256)
		}
		c.Data(200, "application/octet-stream", binaryData)
	})

	// 14. Streaming from bytes.Buffer
	r.GET("/stream/buffer", func(c *router.Context) {
		var buf bytes.Buffer
		for i := 0; i < 100; i++ {
			buf.WriteString(fmt.Sprintf("Line %d\n", i))
		}

		c.DataFromReader(200, int64(buf.Len()), "text/plain", &buf, nil)
	})

	// Info endpoint showing all available rendering methods
	r.GET("/", func(c *router.Context) {
		info := map[string]any{
			"message": "Rivaas Router - Rendering Methods Demo",
			"endpoints": []map[string]string{
				{"path": "/json", "description": "Standard JSON (HTML-escaped)"},
				{"path": "/json/indented", "description": "Pretty-printed JSON"},
				{"path": "/json/pure", "description": "PureJSON (unescaped HTML, 35% faster)"},
				{"path": "/json/secure", "description": "SecureJSON with anti-hijacking prefix"},
				{"path": "/json/ascii", "description": "AsciiJSON (Unicode escaped)"},
				{"path": "/config", "description": "YAML rendering"},
				{"path": "/image", "description": "Binary PNG data"},
				{"path": "/pdf", "description": "PDF document"},
				{"path": "/stream/file", "description": "Stream file with DataFromReader"},
				{"path": "/stream/logs", "description": "Stream text data"},
				{"path": "/jsonp?callback=myFunc", "description": "JSONP callback"},
				{"path": "/benchmark?format=json", "description": "Compare formats"},
				{"path": "/binary", "description": "Raw binary data"},
			},
			"performance": map[string]string{
				"Data()":         "90ns/op - 98% faster (binary/images)",
				"AsciiJSON()":    "1,593ns/op - 62% faster",
				"PureJSON()":     "2,725ns/op - 35% faster (HTML content)",
				"JSON()":         "4,189ns/op - baseline",
				"SecureJSON()":   "4,835ns/op - +15% (compliance)",
				"IndentedJSON()": "8,111ns/op - +94% (debug only)",
				"YAML()":         "36,700ns/op - +776% (config APIs)",
			},
		}

		// Use PureJSON for performance (HTML in descriptions)
		c.PureJSON(200, info)
	})

	port := ":8080"
	fmt.Printf("🚀 Rivaas Router - Rendering Methods Demo\n")
	fmt.Printf("📡 Server running on http://localhost%s\n\n", port)
	fmt.Printf("Try these endpoints:\n")
	fmt.Printf("  curl http://localhost%s/\n", port)
	fmt.Printf("  curl http://localhost%s/json | jq\n", port)
	fmt.Printf("  curl http://localhost%s/json/pure\n", port)
	fmt.Printf("  curl http://localhost%s/json/indented\n", port)
	fmt.Printf("  curl http://localhost%s/config\n", port)
	fmt.Printf("  curl http://localhost%s/benchmark?format=pure\n\n", port)

	log.Fatal(http.ListenAndServe(port, r))
}
