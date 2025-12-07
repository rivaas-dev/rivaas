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

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

// ETag represents an HTTP ETag with optional weak comparison flag.
type ETag struct {
	Value string
	Weak  bool
}

// String returns the ETag in HTTP format (W/"value" for weak, "value" for strong).
func (e ETag) String() string {
	if e.Value == "" {
		return ""
	}
	if e.Weak {
		return `W/"` + e.Value + `"`
	}
	return `"` + e.Value + `"`
}

// WeakETagFromBytes creates a weak ETag from byte data using SHA256.
// Weak ETags allow byte-for-byte comparison but are semantically equivalent
// when content meaning is the same (e.g., different JSON formatting).
func WeakETagFromBytes(b []byte) ETag {
	if len(b) == 0 {
		return ETag{}
	}
	hash := sha256.Sum256(b)
	return ETag{
		Value: hex.EncodeToString(hash[:]),
		Weak:  true,
	}
}

// StrongETagFromBytes creates a strong ETag from byte data using SHA256.
// Strong ETags require exact byte-for-byte matching.
func StrongETagFromBytes(b []byte) ETag {
	if len(b) == 0 {
		return ETag{}
	}
	hash := sha256.Sum256(b)
	return ETag{
		Value: hex.EncodeToString(hash[:]),
		Weak:  false,
	}
}

// WeakETagFromString creates a weak ETag from a string.
func WeakETagFromString(s string) ETag {
	if s == "" {
		return ETag{}
	}
	return WeakETagFromBytes([]byte(s))
}

// StrongETagFromString creates a strong ETag from a string.
func StrongETagFromString(s string) ETag {
	if s == "" {
		return ETag{}
	}
	return StrongETagFromBytes([]byte(s))
}

// CondOpts configures conditional request handling.
// Both ETag and LastModified are checked; if either matches, 304 is returned.
type CondOpts struct {
	ETag         *ETag
	LastModified *time.Time
	Vary         []string // Vary header fields (merged with existing)
}

// SetETag sets the ETag response header.
func (c *Context) SetETag(tag ETag) {
	if tag.Value == "" {
		return
	}
	c.Header("ETag", tag.String())
}

// SetLastModified sets the Last-Modified response header.
func (c *Context) SetLastModified(t time.Time) {
	if t.IsZero() {
		return
	}
	c.Header("Last-Modified", t.UTC().Format(http.TimeFormat))
}

// normalizeETagValue extracts the value from an ETag string (removes quotes and W/ prefix).
func normalizeETagValue(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "W/")
	return strings.Trim(tag, `"`)
}

// HandleConditionals checks If-None-Match and If-Modified-Since headers.
// For safe methods (GET/HEAD), returns 304 Not Modified if client cache is fresh.
// For unsafe methods (PUT/PATCH/DELETE), returns 412 Precondition Failed if preconditions fail.
// Returns true if a response was already written (304 or 412).
//
// IMPORTANT: Call this BEFORE computing expensive response bodies.
//
// Example:
//
//	et := router.WeakETagFromBytes(body)
//	lm := time.Now().UTC().Truncate(time.Second)
//	if c.HandleConditionals(router.CondOpts{ETag: &et, LastModified: &lm}) {
//	    return nil // 304 already written
//	}
//	// Now compute expensive body...
func (c *Context) HandleConditionals(o CondOpts) bool {
	method := c.Request.Method
	isSafe := method == "GET" || method == "HEAD"

	// Handle If-None-Match (takes precedence per RFC 7232)
	if handled, done := c.handleIfNoneMatch(o, isSafe); done {
		return handled
	}

	// Handle If-Modified-Since (only for safe methods)
	if isSafe {
		if handled, done := c.handleIfModifiedSince(o); done {
			return handled
		}
	}

	// Handle If-Match (for unsafe methods)
	if !isSafe {
		if handled, done := c.handleIfMatch(o); done {
			return handled
		}
	}

	// Handle If-Unmodified-Since (for unsafe methods)
	if !isSafe {
		if handled, done := c.handleIfUnmodifiedSince(o); done {
			return handled
		}
	}

	return false
}

// handleIfNoneMatch handles the If-None-Match conditional header per RFC 7232.
// Returns (handled, done) where done=true means processing should stop.
func (c *Context) handleIfNoneMatch(o CondOpts, isSafe bool) (bool, bool) {
	if o.ETag == nil || o.ETag.Value == "" {
		return false, false
	}

	inm := c.Request.Header.Get("If-None-Match")
	if inm == "" {
		return false, false
	}

	normalizedETag := o.ETag.Value
	for tag := range strings.SplitSeq(inm, ",") {
		tag = strings.TrimSpace(tag)

		// Check for wildcard or exact match
		isMatch := tag == "*" || normalizeETagValue(tag) == normalizedETag
		if !isMatch {
			continue
		}

		// Match found - set response headers
		c.SetETag(*o.ETag)
		if len(o.Vary) > 0 {
			c.AddVary(o.Vary...)
		}

		if isSafe {
			c.Status(http.StatusNotModified)
			return true, true
		}

		// For unsafe methods, matching If-None-Match means precondition failed
		reason := "resource exists"
		if tag != "*" {
			reason = "resource unchanged"
		}

		return c.sendPreconditionFailed(reason), true
	}

	return false, false
}

// handleIfModifiedSince handles the If-Modified-Since conditional header per RFC 7232.
func (c *Context) handleIfModifiedSince(o CondOpts) (bool, bool) {
	if o.LastModified == nil || o.LastModified.IsZero() {
		return false, false
	}

	ims := c.Request.Header.Get("If-Modified-Since")
	if ims == "" {
		return false, false
	}

	t, err := http.ParseTime(ims)
	if err != nil || o.LastModified.After(t) {
		return false, false
	}

	c.SetLastModified(*o.LastModified)
	if len(o.Vary) > 0 {
		c.AddVary(o.Vary...)
	}
	c.Status(http.StatusNotModified)

	return true, true
}

// handleIfMatch handles the If-Match conditional header per RFC 7232.
func (c *Context) handleIfMatch(o CondOpts) (bool, bool) {
	if o.ETag == nil || o.ETag.Value == "" {
		return false, false
	}

	im := c.Request.Header.Get("If-Match")
	if im == "" {
		return false, false
	}

	if !etagMatches(im, o.ETag.Value) {
		return c.sendPreconditionFailed("resource modified"), true
	}

	return false, false
}

// handleIfUnmodifiedSince handles the If-Unmodified-Since conditional header per RFC 7232.
func (c *Context) handleIfUnmodifiedSince(o CondOpts) (bool, bool) {
	if o.LastModified == nil || o.LastModified.IsZero() {
		return false, false
	}

	ius := c.Request.Header.Get("If-Unmodified-Since")
	if ius == "" {
		return false, false
	}

	t, err := http.ParseTime(ius)
	if err == nil && o.LastModified.After(t) {
		return c.sendPreconditionFailed("resource modified since"), true
	}

	return false, false
}

// etagMatches checks if the If-Match header matches the given ETag.
func etagMatches(headerValue, etag string) bool {
	for tag := range strings.SplitSeq(headerValue, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "*" || normalizeETagValue(tag) == etag {
			return true
		}
	}

	return false
}

// sendPreconditionFailed sends a 412 Precondition Failed response.
func (c *Context) sendPreconditionFailed(detail string) bool {
	message := "Precondition Failed"
	if detail != "" {
		message += ": " + detail
	}
	c.WriteErrorResponse(http.StatusPreconditionFailed, message)
	return true
}

// IfNoneMatch checks If-None-Match header for safe methods (GET/HEAD).
// Returns true if 304 Not Modified was written.
func (c *Context) IfNoneMatch(tag ETag) bool {
	if tag.Value == "" {
		return false
	}
	method := c.Request.Method
	if method != "GET" && method != "HEAD" {
		return false
	}
	return c.HandleConditionals(CondOpts{ETag: &tag})
}

// IfModifiedSince checks If-Modified-Since header for safe methods (GET/HEAD).
// Returns true if 304 Not Modified was written.
func (c *Context) IfModifiedSince(t time.Time) bool {
	if t.IsZero() {
		return false
	}
	method := c.Request.Method
	if method != "GET" && method != "HEAD" {
		return false
	}
	return c.HandleConditionals(CondOpts{LastModified: &t})
}

// IfMatch checks If-Match header for unsafe methods (PUT/PATCH/DELETE).
// Returns false if precondition failed (412 already sent).
func (c *Context) IfMatch(tag ETag) bool {
	if tag.Value == "" {
		return true // No precondition
	}
	method := c.Request.Method
	if method != "PUT" && method != "PATCH" && method != "DELETE" {
		return true // Not applicable
	}
	im := c.Request.Header.Get("If-Match")
	if im == "" {
		return true // No precondition
	}

	normalizedETag := tag.Value
	for headerTag := range strings.SplitSeq(im, ",") {
		headerTag = strings.TrimSpace(headerTag)
		if headerTag == "*" {
			return true // Match any
		}
		normalizedHeaderTag := normalizeETagValue(headerTag)
		if normalizedHeaderTag == normalizedETag {
			return true // Match found
		}
	}

	// No match found
	return c.sendPreconditionFailed("resource modified")
}

// IfUnmodifiedSince checks If-Unmodified-Since header for unsafe methods.
// Returns false if precondition failed (412 already sent).
func (c *Context) IfUnmodifiedSince(t time.Time) bool {
	if t.IsZero() {
		return true // No precondition
	}
	method := c.Request.Method
	if method != "PUT" && method != "PATCH" && method != "DELETE" {
		return true // Not applicable
	}
	ius := c.Request.Header.Get("If-Unmodified-Since")
	if ius == "" {
		return true // No precondition
	}

	parsed, err := http.ParseTime(ius)
	if err != nil {
		return true // Invalid header, ignore
	}
	if !t.After(parsed) {
		return true // Not modified since
	}

	// Modified since
	return c.sendPreconditionFailed("resource modified since")
}

// AddVary adds fields to the Vary header, deduplicating and normalizing.
// Header names are canonicalized (e.g., "accept" → "Accept").
func (c *Context) AddVary(fields ...string) {
	if len(fields) == 0 {
		return
	}

	// Get existing Vary header
	existing := c.Response.Header().Get("Vary")
	existingFields := make(map[string]bool)
	if existing != "" {
		for field := range strings.SplitSeq(existing, ",") {
			field = strings.TrimSpace(field)
			if field != "" {
				// Normalize to canonical form
				canonical := http.CanonicalHeaderKey(field)
				existingFields[canonical] = true
			}
		}
	}

	// Add new fields (deduplicated)
	for _, field := range fields {
		canonical := http.CanonicalHeaderKey(strings.TrimSpace(field))
		if canonical != "" && !existingFields[canonical] {
			existingFields[canonical] = true
		}
	}

	// Combine existing and new fields
	allFields := make([]string, 0, len(existingFields))
	for field := range existingFields {
		allFields = append(allFields, field)
	}

	// Set Vary header (comma-separated)
	if len(allFields) > 0 {
		c.Header("Vary", strings.Join(allFields, ", "))
	}
}
