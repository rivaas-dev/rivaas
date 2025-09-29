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

// This file contains request information methods for the Context type.
// These methods provide convenient access to request metadata, URLs, headers, and client information.

import (
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// AllParams returns all URL path parameters as a map.
// This is useful for logging, debugging, or when you need all parameters at once.
//
// Returns a new map (copy) to prevent external modification of internal state.
//
// Example:
//
//	// Route: /users/:id/posts/:post_id
//	// Request: /users/123/posts/456
//	params := c.AllParams()
//	// Returns: map[string]string{"id": "123", "post_id": "456"}
func (c *Context) AllParams() map[string]string {
	result := make(map[string]string, c.paramCount)

	// Copy from array storage (≤8 params)
	for i := range c.paramCount {
		result[c.paramKeys[i]] = c.paramValues[i]
	}

	// Copy from map storage (>8 params, rare case)
	maps.Copy(result, c.Params)

	return result
}

// AllQueries returns all query parameters as a map.
// For parameters with multiple values, returns the last value.
// Use c.Request.URL.Query() directly if you need access to all values.
//
// Example:
//
//	// Request: /search?q=golang&page=2&sort=date
//	queries := c.AllQueries()
//	// Returns: map[string]string{"q": "golang", "page": "2", "sort": "date"}
func (c *Context) AllQueries() map[string]string {
	values := c.Request.URL.Query()
	result := make(map[string]string, len(values))

	for key, vals := range values {
		if len(vals) > 0 {
			result[key] = vals[len(vals)-1] // Last value
		}
	}

	return result
}

// RequestHeaders returns all request headers as a map.
// Header names are canonicalized according to HTTP spec (e.g., "user-agent" → "User-Agent").
// For headers with multiple values, returns the last value.
//
// Example:
//
//	headers := c.RequestHeaders()
//	// Returns: map[string]string{"User-Agent": "...", "Accept": "...", ...}
func (c *Context) RequestHeaders() map[string]string {
	headers := c.Request.Header
	result := make(map[string]string, len(headers))

	for key, vals := range headers {
		if len(vals) > 0 {
			result[key] = vals[len(vals)-1]
		}
	}

	return result
}

// ResponseHeaders returns all response headers as a map.
// Useful for debugging, logging, or testing response headers.
//
// Example:
//
//	c.Header("Cache-Control", "no-cache")
//	c.Header("Content-Type", "application/json")
//	headers := c.ResponseHeaders()
//	// Returns: map[string]string{"Cache-Control": "no-cache", "Content-Type": "application/json"}
func (c *Context) ResponseHeaders() map[string]string {
	headers := c.Response.Header()
	result := make(map[string]string, len(headers))

	for key, vals := range headers {
		if len(vals) > 0 {
			result[key] = vals[len(vals)-1]
		}
	}

	return result
}

// Hostname returns the hostname from the Host header.
// For "example.com:8080" returns "example.com".
// For "example.com" returns "example.com".
//
// Example:
//
//	// Host: example.com:8080
//	hostname := c.Hostname() // "example.com"
func (c *Context) Hostname() string {
	host := c.Request.Host
	if host == "" {
		host = c.Request.URL.Host
	}

	// Remove port if present
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		// Check if it's actually a port (not IPv6)
		if !strings.Contains(host, "]") || colonIdx > strings.Index(host, "]") {
			return host[:colonIdx]
		}
	}

	return host
}

// Port returns the port from the Host header.
// Returns empty string if no port is specified.
//
// Example:
//
//	// Host: example.com:8080
//	port := c.Port() // "8080"
//
//	// Host: example.com
//	port := c.Port() // ""
func (c *Context) Port() string {
	host := c.Request.Host
	if host == "" {
		host = c.Request.URL.Host
	}

	// Find port after last colon
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		// Check if it's actually a port (not IPv6)
		if !strings.Contains(host, "]") || colonIdx > strings.Index(host, "]") {
			return host[colonIdx+1:]
		}
	}

	return ""
}

// Scheme returns the request scheme (http or https).
// Checks X-Forwarded-Proto header for proxy scenarios.
//
// Example:
//
//	scheme := c.Scheme() // "https"
func (c *Context) Scheme() string {
	// Check TLS
	if c.Request.TLS != nil {
		return "https"
	}

	// Check X-Forwarded-Proto header (for proxies)
	if proto := c.Request.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}

	// Check X-Forwarded-Ssl header
	if ssl := c.Request.Header.Get("X-Forwarded-Ssl"); ssl == "on" {
		return "https"
	}

	// Default to http
	return "http"
}

// BaseURL returns the base URL (scheme + host).
// Useful for building absolute URLs.
//
// Example:
//
//	// Request: https://example.com:8080/api/users
//	baseURL := c.BaseURL() // "https://example.com:8080"
func (c *Context) BaseURL() string {
	scheme := c.Scheme()
	host := c.Request.Host
	if host == "" {
		host = c.Request.URL.Host
	}

	return scheme + "://" + host
}

// FullURL returns the complete request URL including scheme, host, path, and query string.
// This is the full original request URL.
//
// Example:
//
//	// Request: https://example.com/search?q=golang&page=2
//	fullURL := c.FullURL()
//	// Returns: "https://example.com/search?q=golang&page=2"
func (c *Context) FullURL() string {
	scheme := c.Scheme()
	host := c.Request.Host
	if host == "" {
		host = c.Request.URL.Host
	}

	path := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		path += "?" + c.Request.URL.RawQuery
	}

	return scheme + "://" + host + path
}

// ClientIP returns the real client IP address, respecting trusted proxy headers.
// The implementation is in router/proxies.go and includes security features:
//   - Only trusts headers when the immediate peer is in a trusted CIDR range
//   - Supports X-Forwarded-For, X-Real-IP, and CF-Connecting-IP headers
//   - Prevents IP spoofing attacks
//
// Example:
//
//	clientIP := c.ClientIP() // "203.0.113.1"
//
// See router/proxies.go for the full implementation.

// ClientIPs returns all IP addresses from the X-Forwarded-For chain.
// The first IP is typically the real client, subsequent IPs are proxies.
//
// SECURITY WARNING: Only use this if you trust your proxy infrastructure.
// The X-Forwarded-For header can be spoofed by malicious clients.
// For security-critical decisions, use ClientIP() or validate the IP chain.
//
// Example:
//
//	// X-Forwarded-For: 203.0.113.1, 198.51.100.1, 192.0.2.1
//	ips := c.ClientIPs()
//	// Returns: []string{"203.0.113.1", "198.51.100.1", "192.0.2.1"}
func (c *Context) ClientIPs() []string {
	// Check X-Forwarded-For header
	if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
		// Split by comma and trim spaces
		parts := strings.Split(xff, ",")
		ips := make([]string, 0, len(parts))
		for _, part := range parts {
			if ip := strings.TrimSpace(part); ip != "" {
				ips = append(ips, ip)
			}
		}
		return ips
	}

	// No proxy headers, return direct client IP
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return []string{c.Request.RemoteAddr}
	}
	return []string{ip}
}

// IsJSON returns true if the request content type is application/json.
// This is a helper for content type checking.
//
// Example:
//
//	if c.IsJSON() {
//	    // Parse JSON body
//	}
func (c *Context) IsJSON() bool {
	contentType := c.Request.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}

// IsXML returns true if the request content type is application/xml or text/xml.
// This is a helper for content type checking.
//
// Example:
//
//	if c.IsXML() {
//	    // Parse XML body
//	}
func (c *Context) IsXML() bool {
	contentType := c.Request.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/xml") || strings.Contains(contentType, "text/xml")
}

// AcceptsJSON returns true if the client accepts JSON responses.
// This checks the Accept header for application/json or wildcard.
//
// Example:
//
//	if c.AcceptsJSON() {
//	    c.JSON(http.StatusOK, data)
//	}
func (c *Context) AcceptsJSON() bool {
	accept := c.Request.Header.Get("Accept")
	return strings.Contains(accept, "application/json") || strings.Contains(accept, "*/*")
}

// AcceptsHTML returns true if the client accepts HTML responses.
// This checks the Accept header for text/html or wildcard.
//
// Example:
//
//	if c.AcceptsHTML() {
//	    c.HTML(http.StatusOK, htmlContent)
//	}
func (c *Context) AcceptsHTML() bool {
	accept := c.Request.Header.Get("Accept")
	return strings.Contains(accept, "text/html") || strings.Contains(accept, "*/*")
}

// IsHTTPS returns true if the request is served over HTTPS.
// Checks TLS field, X-Forwarded-Proto, and X-Forwarded-Ssl headers.
//
// Example:
//
//	if c.IsHTTPS() {
//	    // Set secure cookie
//	}
func (c *Context) IsHTTPS() bool {
	return c.Scheme() == "https"
}

// IsLocalhost returns true if the request originates from localhost.
// Checks common localhost representations: 127.0.0.1, ::1, localhost.
//
// Example:
//
//	if c.IsLocalhost() {
//	    // Enable debug features
//	}
func (c *Context) IsLocalhost() bool {
	ip := c.ClientIP()

	// Check common localhost representations
	switch ip {
	case "127.0.0.1", "::1", "localhost", "0.0.0.0", "::":
		return true
	}

	// Check 127.0.0.0/8 range
	if strings.HasPrefix(ip, "127.") {
		return true
	}

	// Check ::1 variations
	if strings.HasPrefix(ip, "::1") || strings.HasPrefix(ip, "0:0:0:0:0:0:0:1") {
		return true
	}

	return false
}

// IsXHR returns true if the request is an AJAX/XMLHttpRequest.
// Checks the X-Requested-With header for "XMLHttpRequest" value.
//
// Modern JavaScript frameworks (jQuery, Axios, etc.) typically set this header
// for AJAX requests, though fetch() API doesn't set it by default.
//
// Example:
//
//	if c.IsXHR() {
//	    // Return JSON instead of HTML
//	}
func (c *Context) IsXHR() bool {
	return c.Request.Header.Get("X-Requested-With") == "XMLHttpRequest"
}

// Subdomains extracts subdomain segments from the Host header.
// The offset parameter specifies how many segments from the end to consider as the main domain.
// Default offset is 2 (assumes "example.com" format).
//
// Examples:
//
//	// Host: api.v1.example.com (offset 2, default)
//	subdomains := c.Subdomains() // []string{"v1", "api"}
//
//	// Host: api.example.com (offset 2, default)
//	subdomains := c.Subdomains() // []string{"api"}
//
//	// Host: api.example.co.uk (offset 3, for .co.uk TLD)
//	subdomains := c.Subdomains(3) // []string{"api"}
//
// SECURITY NOTE: Do not use for authentication or authorization.
// The Host header can be spoofed by clients. Use proper authentication mechanisms.
func (c *Context) Subdomains(offset ...int) []string {
	host := c.Hostname()

	// Default offset is 2 (for example.com)
	off := 2
	if len(offset) > 0 && offset[0] > 0 {
		off = offset[0]
	}

	// Split by dots
	parts := strings.Split(host, ".")

	// If not enough parts, return empty
	if len(parts) <= off {
		return []string{}
	}

	// Return subdomain parts in reverse order (left to right)
	subdomains := parts[:len(parts)-off]

	// Reverse to get left-to-right order
	for i := 0; i < len(subdomains)/2; i++ {
		j := len(subdomains) - 1 - i
		subdomains[i], subdomains[j] = subdomains[j], subdomains[i]
	}

	return subdomains
}

// IsFresh returns true if the response is still fresh in the client's cache.
// Checks If-None-Match (ETag) and If-Modified-Since headers against response headers.
//
// When a client sends Cache-Control: no-cache, this returns false to indicate
// the client wants a full response regardless of cache state.
//
// Based on HTTP conditional request semantics (RFC 7232).
//
// Example:
//
//	if c.IsFresh() {
//	    c.Status(http.StatusNotModified) // 304
//	    return
//	}
//	// Send full response
func (c *Context) IsFresh() bool {
	// Check Cache-Control: no-cache
	cacheControl := c.Request.Header.Get("Cache-Control")
	if strings.Contains(cacheControl, "no-cache") {
		return false
	}

	// Check If-None-Match (ETag validation)
	ifNoneMatch := c.Request.Header.Get("If-None-Match")
	etag := c.Response.Header().Get("ETag")
	if ifNoneMatch != "" && etag != "" {
		// Simple comparison (should handle weak ETags in production)
		if ifNoneMatch == etag || ifNoneMatch == "*" {
			return true
		}
	}

	// Check If-Modified-Since
	ifModifiedSince := c.Request.Header.Get("If-Modified-Since")
	lastModified := c.Response.Header().Get("Last-Modified")
	if ifModifiedSince != "" && lastModified != "" {
		// Simplified check - production should parse dates
		// For now, exact match indicates fresh
		if ifModifiedSince == lastModified {
			return true
		}
	}

	return false
}

// IsStale returns true if the client's cache is stale and a full response should be sent.
// This is the inverse of IsFresh().
//
// Example:
//
//	if c.IsStale() {
//	    // Send full response with updated data
//	}
func (c *Context) IsStale() bool {
	return !c.IsFresh()
}

// FormFile returns the first file for the given form key from a multipart form.
// Returns the multipart.FileHeader which contains filename, size, and content headers.
//
// The caller is responsible for closing the file when done.
//
// Example:
//
//	file, err := c.FormFile("document")
//	if err != nil {
//	    c.JSON(http.StatusBadRequest, map[string]string{"error": "File required"})
//	    return
//	}
//
//	// Use file.Filename, file.Size, file.Header
//	c.SaveFile(file, "./uploads/" + file.Filename)
func (c *Context) FormFile(key string) (*multipart.FileHeader, error) {
	// Parse multipart form if not already parsed
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32 MB max
			return nil, fmt.Errorf("failed to parse multipart form: %w", err)
		}
	}

	// Get file from multipart form
	file, _, err := c.Request.FormFile(key)
	if err != nil {
		return nil, err
	}
	defer func() {
		// Safe to ignore: we've already read the FileHeader metadata we need.
		// The actual file data is accessed later via FileHeader.Open()
		_ = file.Close()
	}()

	// Return the file header
	if c.Request.MultipartForm != nil && c.Request.MultipartForm.File[key] != nil {
		return c.Request.MultipartForm.File[key][0], nil
	}

	return nil, fmt.Errorf("%w: %q", ErrFileNotFound, key)
}

// FormFiles returns all files for the given form key from a multipart form.
// Useful for handling multiple file uploads with the same field name.
//
// Example:
//
//	// HTML: <input type="file" name="documents" multiple>
//	files, err := c.FormFiles("documents")
//	if err != nil {
//	    return err
//	}
//
//	for _, file := range files {
//	    c.SaveFile(file, "./uploads/" + file.Filename)
//	}
func (c *Context) FormFiles(key string) ([]*multipart.FileHeader, error) {
	// Parse multipart form if not already parsed
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32 MB max
			return nil, fmt.Errorf("failed to parse multipart form: %w", err)
		}
	}

	// Get files from multipart form
	if c.Request.MultipartForm != nil && c.Request.MultipartForm.File[key] != nil {
		return c.Request.MultipartForm.File[key], nil
	}

	return nil, fmt.Errorf("%w: %q", ErrNoFilesFound, key)
}

// SaveFile saves an uploaded file to the specified destination path.
// Creates parent directories automatically if they don't exist.
//
// SECURITY WARNING:
//   - Always validate file.Filename before using in paths (prevent path traversal)
//   - Check file.Size to prevent disk exhaustion
//   - Validate file content type to prevent malicious uploads
//   - Consider using random/hashed filenames instead of user-provided names
//
// Example:
//
//	file, err := c.FormFile("document")
//	if err != nil {
//	    return err
//	}
//
//	// Validate
//	if file.Size > 10*1024*1024 {
//	    return errors.New("file too large")
//	}
//
//	// Use safe filename
//	safeName := filepath.Base(file.Filename) // Prevent path traversal
//	err = c.SaveFile(file, "./uploads/" + safeName)
func (c *Context) SaveFile(fileHeader *multipart.FileHeader, dst string) (err error) {
	// Open the uploaded file
	src, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("failed to open uploaded file: %w", err)
	}
	defer func() {
		if cerr := src.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close source file: %w", cerr)
		}
	}()

	// Create parent directories if needed
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		// CRITICAL: Close can fail when flushing buffered data to disk.
		// If Close fails, the file may be incomplete even though io.Copy succeeded.
		if cerr := out.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close destination file: %w", cerr)
		}
	}()

	// Copy file contents
	if _, err := io.Copy(out, src); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// MultipartForm returns the parsed multipart form.
// Useful for advanced form processing or accessing all files at once.
//
// The multipart form is cached after the first call.
//
// Example:
//
//	form, err := c.MultipartForm()
//	if err != nil {
//	    return err
//	}
//
//	// Access all files by field name
//	documents := form.File["documents"]
//	for _, file := range documents {
//	    c.SaveFile(file, "./uploads/" + file.Filename)
//	}
//
//	// Access all form values
//	username := form.Value["username"][0]
func (c *Context) MultipartForm() (*multipart.Form, error) {
	// Parse multipart form if not already parsed
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32 MB max
			return nil, fmt.Errorf("failed to parse multipart form: %w", err)
		}
	}

	return c.Request.MultipartForm, nil
}
