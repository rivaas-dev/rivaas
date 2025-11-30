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

package tracing

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathFilter(t *testing.T) {
	t.Parallel()

	t.Run("NewPathFilter", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		require.NotNil(t, pf)
		assert.NotNil(t, pf.paths)
		assert.Empty(t, pf.prefixes)
		assert.Empty(t, pf.patterns)
	})

	t.Run("NilPathFilter", func(t *testing.T) {
		t.Parallel()

		var pf *pathFilter
		assert.False(t, pf.shouldExclude("/any/path"))
	})
}

func TestPathFilterExactPaths(t *testing.T) {
	t.Parallel()

	t.Run("SinglePath", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPaths("/health")

		assert.True(t, pf.shouldExclude("/health"))
		assert.False(t, pf.shouldExclude("/healthz"))
		assert.False(t, pf.shouldExclude("/api/health"))
	})

	t.Run("MultiplePaths", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPaths("/health", "/ready", "/metrics")

		assert.True(t, pf.shouldExclude("/health"))
		assert.True(t, pf.shouldExclude("/ready"))
		assert.True(t, pf.shouldExclude("/metrics"))
		assert.False(t, pf.shouldExclude("/api/users"))
	})

	t.Run("EmptyPath", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPaths("")

		assert.True(t, pf.shouldExclude(""))
		assert.False(t, pf.shouldExclude("/"))
	})

	t.Run("CaseSensitive", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPaths("/Health")

		assert.True(t, pf.shouldExclude("/Health"))
		assert.False(t, pf.shouldExclude("/health"))
		assert.False(t, pf.shouldExclude("/HEALTH"))
	})
}

func TestPathFilterPrefixes(t *testing.T) {
	t.Parallel()

	t.Run("SinglePrefix", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPrefixes("/api/internal/")

		assert.True(t, pf.shouldExclude("/api/internal/"))
		assert.True(t, pf.shouldExclude("/api/internal/debug"))
		assert.True(t, pf.shouldExclude("/api/internal/metrics/cpu"))
		assert.False(t, pf.shouldExclude("/api/external/"))
		assert.False(t, pf.shouldExclude("/api/internal")) // No trailing slash
	})

	t.Run("MultiplePrefixes", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPrefixes("/debug/", "/internal/", "/_")

		assert.True(t, pf.shouldExclude("/debug/pprof"))
		assert.True(t, pf.shouldExclude("/internal/admin"))
		assert.True(t, pf.shouldExclude("/_health"))
		assert.False(t, pf.shouldExclude("/api/users"))
	})

	t.Run("EmptyPrefix", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPrefixes("")

		// Empty prefix matches everything
		assert.True(t, pf.shouldExclude("/any/path"))
		assert.True(t, pf.shouldExclude(""))
	})

	t.Run("RootPrefix", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPrefixes("/")

		// Root prefix matches everything starting with /
		assert.True(t, pf.shouldExclude("/"))
		assert.True(t, pf.shouldExclude("/api"))
		assert.True(t, pf.shouldExclude("/api/users"))
	})
}

func TestPathFilterPatterns(t *testing.T) {
	t.Parallel()

	t.Run("SinglePattern", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pattern := regexp.MustCompile(`^/api/v\d+/health$`)
		pf.addPatterns(pattern)

		assert.True(t, pf.shouldExclude("/api/v1/health"))
		assert.True(t, pf.shouldExclude("/api/v2/health"))
		assert.True(t, pf.shouldExclude("/api/v99/health"))
		assert.False(t, pf.shouldExclude("/api/v1/users"))
		assert.False(t, pf.shouldExclude("/api/health"))
	})

	t.Run("MultiplePatterns", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPatterns(
			regexp.MustCompile(`^/api/v\d+/health$`),
			regexp.MustCompile(`\.json$`),
			regexp.MustCompile(`^/static/`),
		)

		assert.True(t, pf.shouldExclude("/api/v1/health"))
		assert.True(t, pf.shouldExclude("/data.json"))
		assert.True(t, pf.shouldExclude("/static/js/app.js"))
		assert.False(t, pf.shouldExclude("/api/users"))
	})

	t.Run("WildcardPattern", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pattern := regexp.MustCompile(`.*`)
		pf.addPatterns(pattern)

		// Matches everything
		assert.True(t, pf.shouldExclude("/any/path"))
		assert.True(t, pf.shouldExclude(""))
	})
}

func TestPathFilterCombined(t *testing.T) {
	t.Parallel()

	t.Run("PathsAndPrefixes", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPaths("/health", "/ready")
		pf.addPrefixes("/debug/")

		assert.True(t, pf.shouldExclude("/health"))
		assert.True(t, pf.shouldExclude("/ready"))
		assert.True(t, pf.shouldExclude("/debug/pprof"))
		assert.False(t, pf.shouldExclude("/api/users"))
	})

	t.Run("AllFilterTypes", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPaths("/health")
		pf.addPrefixes("/internal/")
		pf.addPatterns(regexp.MustCompile(`^/api/v\d+/metrics$`))

		// Exact path match
		assert.True(t, pf.shouldExclude("/health"))
		// Prefix match
		assert.True(t, pf.shouldExclude("/internal/debug"))
		// Pattern match
		assert.True(t, pf.shouldExclude("/api/v1/metrics"))
		assert.True(t, pf.shouldExclude("/api/v2/metrics"))
		// No match
		assert.False(t, pf.shouldExclude("/api/users"))
	})

	t.Run("PriorityOrder", func(t *testing.T) {
		t.Parallel()

		// Test that exact paths are checked first (O(1)), then prefixes, then patterns
		pf := newPathFilter()
		pf.addPaths("/api/v1/health")
		pf.addPrefixes("/api/")
		pf.addPatterns(regexp.MustCompile(`.*`))

		// All should match, but the order of checks matters for performance
		assert.True(t, pf.shouldExclude("/api/v1/health")) // Exact match first
		assert.True(t, pf.shouldExclude("/api/users"))     // Prefix match
		assert.True(t, pf.shouldExclude("/other"))         // Pattern match
	})
}

func TestPathFilterEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("SpecialCharactersInPath", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPaths("/path?query=1", "/path#fragment", "/path with spaces")

		assert.True(t, pf.shouldExclude("/path?query=1"))
		assert.True(t, pf.shouldExclude("/path#fragment"))
		assert.True(t, pf.shouldExclude("/path with spaces"))
	})

	t.Run("UnicodeInPath", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPaths("/api/日本語", "/api/émojis/🎉")

		assert.True(t, pf.shouldExclude("/api/日本語"))
		assert.True(t, pf.shouldExclude("/api/émojis/🎉"))
		assert.False(t, pf.shouldExclude("/api/english"))
	})

	t.Run("DuplicatePaths", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPaths("/health", "/health", "/health")

		// Should still work correctly
		assert.True(t, pf.shouldExclude("/health"))
		assert.Len(t, pf.paths, 1) // Map deduplicates
	})

	t.Run("DuplicatePrefixes", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		pf.addPrefixes("/api/", "/api/", "/api/")

		assert.True(t, pf.shouldExclude("/api/users"))
		assert.Len(t, pf.prefixes, 3) // Slice does not deduplicate
	})

	t.Run("EmptyFilter", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()

		// Empty filter should not exclude anything
		assert.False(t, pf.shouldExclude("/any/path"))
		assert.False(t, pf.shouldExclude(""))
		assert.False(t, pf.shouldExclude("/"))
	})

	t.Run("VeryLongPath", func(t *testing.T) {
		t.Parallel()

		pf := newPathFilter()
		longPath := "/" + string(make([]byte, 10000))
		pf.addPaths(longPath)

		assert.True(t, pf.shouldExclude(longPath))
		assert.False(t, pf.shouldExclude("/short"))
	})
}
