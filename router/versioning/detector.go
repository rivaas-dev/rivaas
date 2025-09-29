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

package versioning

import (
	"net/http"
	"strings"
)

// extractQueryVersion scans RawQuery for a specific parameter without parsing.
// This is an alternative to url.Query().Get() for version detection.
//
// Implementation: Manual byte scanning of RawQuery string
// - Looks for "param=" at start or after "&"
// - Extracts value until next "&" or end of string
// - Uses string slicing only
//
// Examples:
//   - "v=v1" → "v1", true
//   - "foo=bar&v=v2&baz=qux" → "v2", true
//   - "version=v1" → "v1", true (if param="version")
//   - "value=v1" → "", false (no match for param="v")
func extractQueryVersion(rawQuery, param string) (string, bool) {
	if rawQuery == "" || param == "" {
		return "", false
	}

	// Build search pattern: "param="
	pattern := param + "="
	patternLen := len(pattern)

	// Search for pattern in query string
	idx := strings.Index(rawQuery, pattern)
	if idx == -1 {
		return "", false
	}

	// Ensure pattern is at a query parameter boundary (start or after "&")
	// This prevents matching "foo=bar" when looking for "oo="
	if idx > 0 && rawQuery[idx-1] != '&' {
		// Not at boundary, search for "&param=" instead
		boundaryPattern := "&" + pattern
		idx = strings.Index(rawQuery, boundaryPattern)
		if idx == -1 {
			return "", false
		}
		idx++ // Skip the '&'
	}

	// Extract value: starts after "param=", ends at next "&" or end of string
	valueStart := idx + patternLen

	// Handle edge case: parameter at end with no value (e.g., "foo=bar&v=")
	// This should return empty string but still indicate parameter was found
	if valueStart >= len(rawQuery) {
		return "", true // Empty value is valid
	}

	// Find end of value (next "&" or end of string)
	valueEnd := strings.IndexByte(rawQuery[valueStart:], '&')
	if valueEnd == -1 {
		// Value extends to end of query string
		return rawQuery[valueStart:], true
	}

	// Value ends at next parameter
	return rawQuery[valueStart : valueStart+valueEnd], true
}

// extractHeaderVersion extracts version from header.
// Uses direct map access instead of Header.Get().
//
// Note: Header names are canonicalized by net/http (e.g., "api-version" → "Api-Version").
// This function expects the canonicalized form.
func extractHeaderVersion(headers http.Header, headerName string) string {
	// Direct map access
	// headers is map[string][]string, we want the first value
	if vals, ok := headers[headerName]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// extractPathVersion extracts version from URL path.
// Uses string operations to match a path pattern like "/v{version}/" or "/api/{version}/"
//
// Implementation: Prefix matching and segment extraction
// - Checks if path starts with configured prefix (e.g., "/v")
// - Extracts next path segment as version
// - Stops at next "/" or end of string
//
// Examples (with pattern "/v{version}/"):
//   - "/v1/users" → "v1", true
//   - "/v2/posts/123" → "v2", true
//   - "/api/v1/users" → "", false (doesn't match prefix)
//
// Examples (with pattern "/api/v{version}/"):
//   - "/api/v1/users" → "v1", true
//   - "/api/v2/posts" → "v2", true
func extractPathVersion(path, prefix string) (string, bool) {
	if path == "" || prefix == "" {
		return "", false
	}

	// Check if path starts with prefix
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}

	// Extract version segment after prefix
	// Skip the prefix to get to version start
	versionStart := len(prefix)
	if versionStart >= len(path) {
		return "", false
	}

	// Find end of version segment (next "/" or end of path)
	remaining := path[versionStart:]
	versionEnd := strings.IndexByte(remaining, '/')

	if versionEnd == -1 {
		// Version extends to end of path (e.g., "/v1")
		return remaining, true
	}

	// Version ends at next segment (e.g., "/v1/users" -> "v1")
	return remaining[:versionEnd], true
}

// extractAcceptVersion extracts version from Accept header using content negotiation.
// Parses vendor-specific media types like "application/vnd.myapi.v2+json"
//
// Implementation: Pattern matching with {version} placeholder
// - Locates the pattern prefix in Accept header
// - Extracts version between prefix and suffix
// - Handles multiple Accept values (comma-separated)
//
// Examples (with pattern "application/vnd.myapi.v{version}+json"):
//   - "application/vnd.myapi.v2+json" → "v2", true
//   - "application/vnd.myapi.v1+json, text/html" → "v1", true
//   - "application/json" → "", false
func extractAcceptVersion(accept, pattern string) (string, bool) {
	if accept == "" || pattern == "" {
		return "", false
	}

	// Find {version} placeholder in pattern
	versionIdx := strings.Index(pattern, "{version}")
	if versionIdx == -1 {
		return "", false
	}

	// Split pattern into prefix and suffix
	prefix := pattern[:versionIdx]
	suffix := pattern[versionIdx+9:] // len("{version}") == 9

	// Handle comma-separated Accept values
	// e.g., "application/vnd.myapi.v2+json, text/html"
	for val := range strings.SplitSeq(accept, ",") {
		val = strings.TrimSpace(val)

		// Find prefix in this Accept value
		prefixIdx := strings.Index(val, prefix)
		if prefixIdx == -1 {
			continue
		}

		// Extract version between prefix and suffix
		versionStart := prefixIdx + len(prefix)
		if suffix == "" {
			// No suffix, version extends to end or semicolon (for parameters)
			versionEnd := strings.IndexAny(val[versionStart:], ";,")
			if versionEnd == -1 {
				return strings.TrimSpace(val[versionStart:]), true
			}
			return strings.TrimSpace(val[versionStart : versionStart+versionEnd]), true
		}

		// Find suffix after version
		suffixIdx := strings.Index(val[versionStart:], suffix)
		if suffixIdx == -1 {
			continue
		}

		return val[versionStart : versionStart+suffixIdx], true
	}

	return "", false
}
