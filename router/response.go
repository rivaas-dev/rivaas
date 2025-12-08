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

// This file contains advanced response helper methods for the Context type.
// These methods provide convenient ways to set headers, send files, and format responses.

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
)

// AppendHeader appends a value to an existing response header.
// If the header doesn't exist, it creates it with the given value.
// This is useful for headers that can have multiple values like Set-Cookie or Link.
//
// Example:
//
//	c.AppendHeader("Set-Cookie", "session=abc; HttpOnly")
//	c.AppendHeader("Set-Cookie", "theme=dark; Path=/")
//	// Results in two Set-Cookie headers
func (c *Context) AppendHeader(key, value string) {
	existing := c.Response.Header().Get(key)
	if existing == "" {
		c.Header(key, value)
	} else {
		c.Response.Header().Set(key, existing+", "+value)
	}
}

// ContentType sets the Content-Type header.
// Accepts both file extensions (.json, .html) and full MIME types.
//
// Example:
//
//	c.ContentType(".json")           // Sets "application/json"
//	c.ContentType("json")            // Sets "application/json"
//	c.ContentType("application/xml") // Sets "application/xml"
func (c *Context) ContentType(value string) {
	// If it's already a MIME type, use it directly
	if strings.Contains(value, "/") {
		c.Header("Content-Type", value)
		return
	}

	// Ensure it starts with a dot for mime.TypeByExtension
	if !strings.HasPrefix(value, ".") {
		value = "." + value
	}

	// Get MIME type from extension
	mimeType := mime.TypeByExtension(value)
	if mimeType == "" {
		// Fallback to common types not in mime package
		switch value {
		case ".json":
			mimeType = "application/json"
		case ".html", ".htm":
			mimeType = "text/html"
		case ".xml":
			mimeType = "application/xml"
		case ".txt":
			mimeType = "text/plain"
		default:
			mimeType = "application/octet-stream"
		}
	}

	c.Header("Content-Type", mimeType)
}

// Location sets the Location header for redirects.
// This is typically used before calling Status() with 3xx codes.
//
// Example:
//
//	c.Location("/new-path")
//	c.Status(http.StatusMovedPermanently)
func (c *Context) Location(url string) {
	c.Header("Location", url)
}

// Vary adds fields to the Vary response header.
// The Vary header tells caches which request headers affect the response.
// Multiple calls append additional fields.
//
// Example:
//
//	c.Vary("Accept-Encoding")
//	c.Vary("Accept-Language", "Cookie")
//	// Vary: Accept-Encoding, Accept-Language, Cookie
func (c *Context) Vary(fields ...string) {
	existing := c.Response.Header().Get("Vary")

	for _, field := range fields {
		// Skip if already present
		if existing != "" && strings.Contains(existing, field) {
			continue
		}

		if existing == "" {
			existing = field
		} else {
			existing += ", " + field
		}
	}

	c.Response.Header().Set("Vary", existing)
}

// Link adds a Link header for resource relationships.
// Follows RFC 8288 web linking format.
//
// Example:
//
//	c.Link("/api/users?page=2", "next")
//	c.Link("/api/users?page=5", "last")
//	// Link: </api/users?page=2>; rel="next", </api/users?page=5>; rel="last"
func (c *Context) Link(url, rel string) {
	linkValue := fmt.Sprintf("<%s>; rel=\"%s\"", url, rel)
	c.AppendHeader("Link", linkValue)
}

// Download transfers a file as a downloadable attachment.
// Sets Content-Disposition header and serves the file.
// Optional filename parameter overrides the actual filename shown to user.
//
// Example:
//
//	c.Download("./uploads/report-12345.pdf")                    // Downloads as "report-12345.pdf"
//	c.Download("./uploads/report-12345.pdf", "monthly-report.pdf") // Downloads as "monthly-report.pdf"
func (c *Context) Download(filepath string, filename ...string) error {
	// Determine filename for Content-Disposition
	var downloadName string
	if len(filename) > 0 && filename[0] != "" {
		downloadName = filename[0]
	} else {
		downloadName = filepath[strings.LastIndex(filepath, "/")+1:]
		if downloadName == "" {
			downloadName = "download"
		}
	}

	// Set Content-Disposition header
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadName))

	// Serve the file
	http.ServeFile(c.Response, c.Request, filepath)

	return nil
}

// Send sends raw bytes as the response body.
// Sets Content-Type to application/octet-stream if not already set.
//
// Example:
//
//	data := []byte("raw binary data")
//	c.Send(data)
func (c *Context) Send(data []byte) error {
	if c.Response.Header().Get("Content-Type") == "" {
		c.Header("Content-Type", "application/octet-stream")
	}

	_, err := c.Response.Write(data)

	return err
}

// SendStatus sends an HTTP status code with the standard status text as body.
// If the response body is already written, only sets the status.
//
// Example:
//
//	c.SendStatus(http.StatusNotFound) // Sends "404 Not Found"
//	c.SendStatus(http.StatusCreated) // Sends "201 Created"
func (c *Context) SendStatus(code int) error {
	c.Status(code)

	// Check if body already written
	if rw, ok := c.Response.(*responseWriter); ok {
		if rw.Written() {
			return nil
		}
	}

	// Send status text as body
	statusText := http.StatusText(code)
	if statusText == "" {
		statusText = fmt.Sprintf("%d Status Code", code)
	}

	_, err := c.Response.Write([]byte(statusText))

	return err
}

// JSONP sends a JSON response with JSONP callback support.
// The callback parameter wraps the JSON in a function call for cross-domain requests.
// Default callback name is "callback" if not specified.
// Returns an error if encoding or writing fails.
//
// Example:
//
//	if err := c.JSONP(http.StatusOK, data); err != nil {
//	    return err
//	}
//	c.JSONP(http.StatusOK, data, "myFunc")    // myFunc({"key": "value"})
func (c *Context) JSONP(code int, obj any, callback ...string) error {
	// Determine callback name
	callbackName := "callback"
	if len(callback) > 0 && callback[0] != "" {
		callbackName = callback[0]
	}

	// Marshal JSON
	jsonData, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("JSONP encoding failed: %w", err)
	}

	// Set headers
	c.Header("Content-Type", "application/javascript; charset=utf-8")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Status(code)

	// Send JSONP response: callback(json)
	response := fmt.Sprintf("%s(%s)", callbackName, jsonData)
	_, writeErr := c.Response.Write([]byte(response))

	return writeErr
}

// Format performs automatic content negotiation and sends the response.
// Chooses format based on Accept header: JSON, HTML, XML, or plain text.
//
// Example:
//
//	c.Format(http.StatusOK, user)
//	// Accept: application/json → sends JSON
//	// Accept: text/html → sends HTML representation
//	// Accept: text/plain → sends string representation
func (c *Context) Format(code int, data any) error {
	// Use existing Accepts() method for negotiation
	format := c.Accepts("json", "html", "xml", "txt")

	switch format {
	case "json":
		return c.JSON(code, data)

	case "html":
		// Try to convert to HTML
		html := fmt.Sprintf("<p>%v</p>", data)
		return c.HTML(code, html)

	case "xml":
		// Simple XML formatting
		c.Header("Content-Type", "application/xml")
		c.Status(code)
		xml := fmt.Sprintf("<?xml version=\"1.0\"?>\n<response>%v</response>", data)
		_, err := c.Response.Write([]byte(xml))

		return err

	case "txt", "":
		// Plain text fallback
		return c.Stringf(code, "%v", data)

	default:
		// Default to JSON
		return c.JSON(code, data)
	}
}

// Write implements the io.Writer interface.
// Writes raw bytes to the response body.
// This allows using Context with fmt.Fprintf and other io.Writer consumers.
//
// Example:
//
//	fmt.Fprintf(c, "User: %s, Score: %d", name, score)
func (c *Context) Write(data []byte) (int, error) {
	return c.Response.Write(data)
}

// WriteStringBody writes a string directly to the response body.
// This is for low-level writing after headers are already set.
//
// For sending HTTP responses with status codes, use String() or WriteString() instead.
//
// Example:
//
//	c.WriteStringBody("Hello, World!")
//	c.WriteStringBody(fmt.Sprintf("Count: %d", count))
func (c *Context) WriteStringBody(str string) (int, error) {
	return io.WriteString(c.Response, str)
}
