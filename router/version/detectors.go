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

package version

import (
	"net/http"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Path Detector
// ═══════════════════════════════════════════════════════════════════════════════

type pathDetector struct {
	pattern string
	prefix  string // Extracted prefix before {version}
}

func newPathDetector(pattern string) *pathDetector {
	idx := strings.Index(pattern, "{version}")
	prefix := ""
	if idx > 0 {
		prefix = pattern[:idx]
	}

	return &pathDetector{
		pattern: pattern,
		prefix:  prefix,
	}
}

func (d *pathDetector) Detect(req *http.Request) (string, bool) {
	if req == nil || req.URL == nil {
		return "", false
	}

	return d.extractFromPath(req.URL.Path)
}

func (d *pathDetector) Method() string {
	return "path"
}

// Pattern returns the original pattern (for path stripping).
func (d *pathDetector) Pattern() string {
	return d.pattern
}

// Prefix returns the prefix before {version} (for path stripping).
func (d *pathDetector) Prefix() string {
	return d.prefix
}

func (d *pathDetector) extractFromPath(path string) (string, bool) {
	if d.prefix == "" || !strings.HasPrefix(path, d.prefix) {
		return "", false
	}

	// Extract version segment after prefix
	remaining := path[len(d.prefix):]
	if remaining == "" {
		return "", false
	}

	// Find end of version segment (next "/" or end)
	end := strings.IndexByte(remaining, '/')
	var segment string
	if end == -1 {
		segment = remaining
	} else {
		segment = remaining[:end]
	}

	if segment == "" {
		return "", false
	}

	// Handle "v" prefix in pattern
	// e.g., pattern "/v{version}/" with path "/v1/users" extracts "1"
	// We return "v1" for better matching with version trees
	if strings.HasSuffix(d.prefix, "v") {
		return "v" + segment, true
	}

	return segment, true
}

// ExtractSegment extracts the raw version segment for path stripping.
// This includes any prefix characters like "v" in "/v1/users".
func (d *pathDetector) ExtractSegment(path string) (string, bool) {
	if d.prefix == "" || !strings.HasPrefix(path, d.prefix) {
		return "", false
	}

	remaining := path[len(d.prefix):]
	if remaining == "" {
		return "", false
	}

	end := strings.IndexByte(remaining, '/')
	var segment string
	if end == -1 {
		segment = remaining
	} else {
		segment = remaining[:end]
	}

	if segment == "" {
		return "", false
	}

	// Return with "v" prefix if pattern uses it
	if strings.HasSuffix(d.prefix, "v") {
		return "v" + segment, true
	}

	return segment, true
}

// StripVersion removes the version segment from the path.
func (d *pathDetector) StripVersion(path, version string) string {
	if !strings.HasPrefix(path, d.prefix) {
		return path
	}

	prefixLen := len(d.prefix)
	if prefixLen >= len(path) {
		return path
	}

	remaining := path[prefixLen:]
	end := strings.IndexByte(remaining, '/')

	if end == -1 {
		// Version is at end of path
		return "/"
	}

	// Return path from the "/" after version segment
	return remaining[end:]
}

// ═══════════════════════════════════════════════════════════════════════════════
// Header Detector
// ═══════════════════════════════════════════════════════════════════════════════

type headerDetector struct {
	header string
}

func (d *headerDetector) Detect(req *http.Request) (string, bool) {
	if req == nil {
		return "", false
	}
	v := req.Header.Get(d.header)

	return v, v != ""
}

func (d *headerDetector) Method() string {
	return "header"
}

// ═══════════════════════════════════════════════════════════════════════════════
// Query Detector
// ═══════════════════════════════════════════════════════════════════════════════

type queryDetector struct {
	param string
}

func (d *queryDetector) Detect(req *http.Request) (string, bool) {
	if req == nil || req.URL == nil {
		return "", false
	}

	return d.extractFromQuery(req.URL.RawQuery)
}

func (d *queryDetector) Method() string {
	return "query"
}

func (d *queryDetector) extractFromQuery(query string) (string, bool) {
	if query == "" {
		return "", false
	}

	// Fast path: look for param=value pattern
	prefix := d.param + "="
	idx := strings.Index(query, prefix)
	if idx == -1 {
		return "", false
	}

	// Check if it's at start or after &
	if idx > 0 && query[idx-1] != '&' {
		// Might be a substring match (e.g., "api_version" matching "version")
		// Search again after the current position
		rest := query[idx+1:]
		newIdx := strings.Index(rest, prefix)
		if newIdx == -1 {
			return "", false
		}
		if rest[newIdx-1] != '&' {
			return "", false
		}
		idx = idx + 1 + newIdx
	}

	// Extract value
	start := idx + len(prefix)
	end := strings.IndexByte(query[start:], '&')
	if end == -1 {
		return query[start:], true
	}

	return query[start : start+end], true
}

// ═══════════════════════════════════════════════════════════════════════════════
// Accept Detector
// ═══════════════════════════════════════════════════════════════════════════════

type acceptDetector struct {
	pattern string
	prefix  string // Part before {version}
	suffix  string // Part after {version}
}

func (d *acceptDetector) Detect(req *http.Request) (string, bool) {
	if req == nil {
		return "", false
	}

	// Parse pattern on first use
	if d.prefix == "" && d.suffix == "" {
		idx := strings.Index(d.pattern, "{version}")
		if idx >= 0 {
			d.prefix = d.pattern[:idx]
			d.suffix = d.pattern[idx+9:] // len("{version}") = 9
		}
	}

	accept := req.Header.Get("Accept")
	if accept == "" {
		return "", false
	}

	return d.extractFromAccept(accept)
}

func (d *acceptDetector) Method() string {
	return "accept"
}

func (d *acceptDetector) extractFromAccept(accept string) (string, bool) {
	// Handle multiple media types
	for mediaType := range strings.SplitSeq(accept, ",") {
		mediaType = strings.TrimSpace(mediaType)

		// Remove quality parameter if present
		if semi := strings.IndexByte(mediaType, ';'); semi >= 0 {
			mediaType = mediaType[:semi]
		}

		if !strings.HasPrefix(mediaType, d.prefix) {
			continue
		}
		if !strings.HasSuffix(mediaType, d.suffix) {
			continue
		}

		// Extract version between prefix and suffix
		version := mediaType[len(d.prefix) : len(mediaType)-len(d.suffix)]
		if version != "" {
			return version, true
		}
	}

	return "", false
}

type customDetector struct {
	fn func(*http.Request) string
}

func (d *customDetector) Detect(req *http.Request) (string, bool) {
	if d.fn == nil {
		return "", false
	}
	v := d.fn(req)

	return v, v != ""
}

func (d *customDetector) Method() string {
	return "custom"
}
