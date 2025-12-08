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

package compiler

// ContextParamWriter is an interface for writing route parameters to a context.
// This avoids import cycles by not importing router.Context directly.
type ContextParamWriter interface {
	SetParam(index int, key, value string)
	SetParamMap(key, value string)
	SetParamCount(count int32)
}

// MatchDynamic attempts to match path against dynamic routes.
// Uses first-segment index for filtering.
func (rc *RouteCompiler) MatchDynamic(method, path string, ctx ContextParamWriter) *CompiledRoute {
	rc.mu.RLock()

	// Build first-segment index if not exists (lazy initialization)
	if !rc.hasFirstSegmentIndex && len(rc.dynamicRoutes) > minRoutesForIndexing {
		// Only build index if we have enough routes to benefit
		rc.mu.RUnlock()
		rc.buildFirstSegmentIndex()
		rc.mu.RLock()
	}

	// Try first-segment index for filtering
	if rc.hasFirstSegmentIndex && len(path) > 1 {
		// Extract first character after '/'
		firstChar := path[1]
		if firstChar < 128 {
			// ASCII path - check jump table
			// Note: Non-ASCII paths (UTF-8 beyond byte 127) skip this index
			// and fall back to linear scan. This is intentional - see struct comment.
			candidates := rc.firstSegmentIndex[firstChar]
			for _, route := range candidates {
				// Check method before matching path
				if route.method == method && route.matchAndExtract(path, ctx) {
					rc.mu.RUnlock()
					return route
				}
			}
			rc.mu.RUnlock()

			return nil
		}
	}

	// Fallback: Try each route in order (sorted by specificity)
	for _, route := range rc.dynamicRoutes {
		// Check method before matching path
		if route.method == method && route.matchAndExtract(path, ctx) {
			rc.mu.RUnlock()
			return route
		}
	}

	rc.mu.RUnlock()

	return nil
}

// buildFirstSegmentIndex builds the first-segment jump table for route filtering.
// This reduces the search space for typical APIs.
//
// Instead of checking all routes, only check those that start with
// the same character as the path.
//
// Example: Path "/users/123" → Only check routes starting with 'u'
//
// DESIGN DECISION: Only indexes ASCII characters (0-127).
// UTF-8 paths beyond ASCII are still matched correctly via fallback linear scan.
// Extending to full UTF-8 would add complexity without benefit for
// typical HTTP APIs where ASCII paths dominate.
func (rc *RouteCompiler) buildFirstSegmentIndex() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Reset index
	for i := range rc.firstSegmentIndex {
		rc.firstSegmentIndex[i] = nil
	}

	// Build index from routes
	for _, route := range rc.dynamicRoutes {
		// Get first character of pattern after '/'
		pattern := route.pattern
		if len(pattern) > 1 && pattern[0] == '/' {
			firstChar := pattern[1]
			if firstChar < 128 {
				// Add to index
				rc.firstSegmentIndex[firstChar] = append(rc.firstSegmentIndex[firstChar], route)
			}
		}
	}

	rc.hasFirstSegmentIndex = true
}

// matchAndExtract attempts to match a path against this compiled route and extract parameters.
// Uses early exit for common patterns.
//
// Implementation: Single-pass validation and extraction
// 1. Rejection by segment count
// 2. Parse path into stack-allocated array
// 3. Validate static segments by direct position check
// 4. Extract parameters by position with inline constraint validation
func (r *CompiledRoute) matchAndExtract(path string, ctx ContextParamWriter) bool {
	// Handle root path specially (unlikely in most APIs)
	if r.segmentCount == 0 {
		return path == "/" || path == ""
	}

	// Common single-parameter routes
	// Pattern: /resource/:id (2 segments, 1 param at position 1)
	// This is the most common REST API pattern
	if r.segmentCount == 2 && len(r.paramPos) == 1 && r.paramPos[0] == 1 {
		// Common case: /users/123, /posts/456, etc.
		// No need for full parsing - just extract after first /
		if len(path) < 3 || path[0] != '/' {
			return false // Too short or invalid
		}

		// Find first segment boundary
		firstSlash := -1
		for i := 1; i < len(path); i++ {
			if path[i] == '/' {
				firstSlash = i
				break
			}
		}

		if firstSlash == -1 {
			return false // No second segment
		}

		// Check if there's a third segment (shouldn't be)
		for i := firstSlash + 1; i < len(path); i++ {
			if path[i] == '/' {
				return false // Too many segments
			}
		}

		// Validate static segment
		firstSeg := path[1:firstSlash]
		if len(r.staticSegments) > 0 && firstSeg != r.staticSegments[0] {
			return false
		}

		// Extract parameter (second segment)
		paramValue := path[firstSlash+1:]

		// Inline constraint check
		if len(r.constraints) > 0 && r.constraints[0] != nil {
			if !r.constraints[0].MatchString(paramValue) {
				return false
			}
		}

		// Store in context
		ctx.SetParam(0, r.paramNames[0], paramValue)
		ctx.SetParamCount(1)

		return true
	}

	pathLen := len(path)

	// Length check for early exit
	// If path is too short, it can't possibly match
	// Minimum length = segmentCount + (segmentCount - 1) slashes
	minLen := int(r.segmentCount) + int(r.segmentCount-1)
	if pathLen < minLen {
		return false
	}

	// Count slashes with early exit
	// Check segment count matches while counting
	slashCount := int32(0)
	for i := range pathLen {
		if path[i] == '/' {
			slashCount++
			// Early exit if too many slashes
			if slashCount > r.segmentCount {
				return false
			}
		}
	}

	// Exact slash count validation
	expectedSlashes := r.segmentCount
	if pathLen == 0 || path[0] != '/' {
		expectedSlashes-- // Pattern has leading / but path doesn't
	}

	if slashCount != expectedSlashes {
		return false
	}

	// Stack-allocated segment buffer
	// 16 segments should be enough for any reasonable route
	var segments [16]string
	segCount := int32(0)

	// Single-pass path parsing
	start := 0
	if len(path) > 0 && path[0] == '/' {
		start = 1 // Skip leading slash
	}

	for start < pathLen && segCount < 16 {
		end := start
		for end < pathLen && path[end] != '/' {
			end++
		}
		segments[segCount] = path[start:end]
		segCount++
		start = end + 1
	}

	// Validate segment count matches
	if segCount != r.segmentCount {
		return false
	}

	// Validate static segments with early exit
	// Check most distinctive segments first for early rejection
	staticCount := int32(len(r.staticPos))
	if staticCount > 0 {
		// Unroll first check (most common case)
		pos0 := r.staticPos[0]
		if pos0 >= segCount || segments[pos0] != r.staticSegments[0] {
			return false
		}

		// Check remaining static segments
		for i := int32(1); i < staticCount; i++ {
			pos := r.staticPos[i]
			if pos >= segCount || segments[pos] != r.staticSegments[i] {
				return false
			}
		}
	}

	// Extract and validate parameters (by position - no search!)
	paramCount := int32(len(r.paramPos))
	for i := range int(paramCount) {
		pos := r.paramPos[i]
		if pos >= segCount {
			return false
		}

		value := segments[pos]

		// Inline constraint validation (if constraint exists)
		if int32(i) < int32(len(r.constraints)) && r.constraints[int32(i)] != nil {
			if !r.constraints[int32(i)].MatchString(value) {
				return false
			}
		}

		// Store parameter in context (we already know the index!)
		if i < 8 {
			ctx.SetParam(i, r.paramNames[i], value)
		} else {
			// Rare case: >8 parameters
			ctx.SetParamMap(r.paramNames[i], value)
		}
	}

	ctx.SetParamCount(paramCount)

	return true
}
