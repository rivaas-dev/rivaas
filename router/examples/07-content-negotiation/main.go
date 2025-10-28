package main

import (
	"fmt"
	"log"
	"net/http"

	"rivaas.dev/router"
)

type User struct {
	ID   int    `json:"id" xml:"id"`
	Name string `json:"name" xml:"name"`
}

func main() {
	r := router.New()

	// Example 1: Content negotiation with Accepts()
	r.GET("/api/user", func(c *router.Context) {
		user := User{ID: 1, Name: "John Doe"}

		// Negotiate response format based on Accept header
		switch c.Accepts("json", "xml", "html") {
		case "json":
			c.JSON(http.StatusOK, user)
		case "xml":
			c.Header("Content-Type", "application/xml")
			c.String(http.StatusOK, "<user><id>%d</id><name>%s</name></user>", user.ID, user.Name)
		case "html":
			html := fmt.Sprintf("<html><body><h1>User: %s</h1><p>ID: %d</p></body></html>", user.Name, user.ID)
			c.HTML(http.StatusOK, html)
		default:
			c.Status(http.StatusNotAcceptable)
			c.String(http.StatusNotAcceptable, "Not Acceptable")
		}
	})

	// Example 2: Language negotiation
	r.GET("/api/greeting", func(c *router.Context) {
		greetings := map[string]string{
			"en": "Hello!",
			"fr": "Bonjour!",
			"de": "Hallo!",
			"es": "¡Hola!",
		}

		lang := c.AcceptsLanguages("en", "fr", "de", "es")
		if lang == "" {
			lang = "en" // fallback
		}

		c.JSON(http.StatusOK, map[string]string{
			"language": lang,
			"greeting": greetings[lang],
		})
	})

	// Example 3: Encoding negotiation for compression
	r.GET("/api/data", func(c *router.Context) {
		data := map[string]any{
			"message": "This is sample data that could be compressed",
			"items":   []int{1, 2, 3, 4, 5},
		}

		encoding := c.AcceptsEncodings("gzip", "br", "deflate")
		if encoding != "" {
			c.Header("Content-Encoding", encoding)
			// In production, you would actually compress the response here
			c.Header("X-Encoding-Applied", encoding)
		}

		c.JSON(http.StatusOK, data)
	})

	// Example 4: Charset negotiation
	r.GET("/api/text", func(c *router.Context) {
		charset := c.AcceptsCharsets("utf-8", "iso-8859-1", "ascii")
		if charset == "" {
			charset = "utf-8" // fallback
		}

		c.Header("Content-Type", fmt.Sprintf("text/plain; charset=%s", charset))
		c.String(http.StatusOK, "Sample text content")
	})

	// Example 5: Combined negotiation
	r.GET("/api/flexible", func(c *router.Context) {
		// Check what the client accepts
		format := c.Accepts("json", "xml", "html", "txt")
		encoding := c.AcceptsEncodings("gzip", "br", "deflate")
		lang := c.AcceptsLanguages("en", "fr", "de")

		response := map[string]string{
			"format":   format,
			"encoding": encoding,
			"language": lang,
			"message":  "Content negotiation successful",
		}

		c.JSON(http.StatusOK, response)
	})

	// Example 6: API versioning with content negotiation
	r.GET("/api/resource", func(c *router.Context) {
		// Prefer JSON but support other formats
		format := c.Accepts("json", "xml")
		if format == "" {
			c.Status(http.StatusNotAcceptable)
			c.JSON(http.StatusNotAcceptable, map[string]string{
				"error": "Supported formats: application/json, application/xml",
			})
			return
		}

		data := map[string]any{
			"id":   123,
			"type": "resource",
		}

		if format == "json" {
			c.JSON(http.StatusOK, data)
		} else {
			c.Header("Content-Type", "application/xml")
			c.String(http.StatusOK, "<resource><id>123</id><type>resource</type></resource>")
		}
	})

	log.Println("Server starting on :8080")
	log.Println("\nTry these curl commands:")
	log.Println("  curl -H 'Accept: application/json' http://localhost:8080/api/user")
	log.Println("  curl -H 'Accept: application/xml' http://localhost:8080/api/user")
	log.Println("  curl -H 'Accept: text/html' http://localhost:8080/api/user")
	log.Println("  curl -H 'Accept-Language: fr' http://localhost:8080/api/greeting")
	log.Println("  curl -H 'Accept-Encoding: br' http://localhost:8080/api/data")
	log.Println("  curl -H 'Accept: application/json' -H 'Accept-Language: de' -H 'Accept-Encoding: gzip' http://localhost:8080/api/flexible")

	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}
}
