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

package metrics

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathFilter_ExactPaths(t *testing.T) {
	t.Parallel()

	pf := newPathFilter()
	pf.addPaths("/health", "/metrics", "/ready")

	assert.True(t, pf.shouldExclude("/health"))
	assert.True(t, pf.shouldExclude("/metrics"))
	assert.True(t, pf.shouldExclude("/ready"))
	assert.False(t, pf.shouldExclude("/api/users"))
	assert.False(t, pf.shouldExclude("/health/check")) // Not exact match
}

func TestPathFilter_Prefixes(t *testing.T) {
	t.Parallel()

	pf := newPathFilter()
	pf.addPrefixes("/debug/", "/internal/")

	assert.True(t, pf.shouldExclude("/debug/pprof"))
	assert.True(t, pf.shouldExclude("/debug/vars"))
	assert.True(t, pf.shouldExclude("/internal/status"))
	assert.True(t, pf.shouldExclude("/internal/config/reload"))
	assert.False(t, pf.shouldExclude("/api/users"))
	assert.False(t, pf.shouldExclude("/debugger")) // Doesn't start with /debug/
}

func TestPathFilter_Patterns(t *testing.T) {
	t.Parallel()

	pf := newPathFilter()
	pattern1, err := regexp.Compile(`^/v[0-9]+/internal/.*`)
	require.NoError(t, err)
	pattern2, err := regexp.Compile(`^/admin/.*`)
	require.NoError(t, err)
	pf.addPatterns(pattern1, pattern2)

	assert.True(t, pf.shouldExclude("/v1/internal/status"))
	assert.True(t, pf.shouldExclude("/v2/internal/config"))
	assert.True(t, pf.shouldExclude("/admin/users"))
	assert.True(t, pf.shouldExclude("/admin/settings/global"))
	assert.False(t, pf.shouldExclude("/api/users"))
	assert.False(t, pf.shouldExclude("/v1/users")) // Doesn't match internal
}

func TestPathFilter_Combined(t *testing.T) {
	t.Parallel()

	pf := newPathFilter()
	pf.addPaths("/health", "/ready")
	pf.addPrefixes("/debug/")
	pattern, err := regexp.Compile(`^/v[0-9]+/internal/.*`)
	require.NoError(t, err)
	pf.addPatterns(pattern)

	// Exact paths
	assert.True(t, pf.shouldExclude("/health"))
	assert.True(t, pf.shouldExclude("/ready"))

	// Prefixes
	assert.True(t, pf.shouldExclude("/debug/pprof"))

	// Patterns
	assert.True(t, pf.shouldExclude("/v1/internal/status"))

	// Not excluded
	assert.False(t, pf.shouldExclude("/api/users"))
}

func TestPathFilter_EmptyConfig(t *testing.T) {
	t.Parallel()

	pf := newPathFilter()

	// With no exclusions configured, nothing should be excluded
	assert.False(t, pf.shouldExclude("/health"))
	assert.False(t, pf.shouldExclude("/api/users"))
}

func TestPathFilter_NilFilter(t *testing.T) {
	t.Parallel()

	var pf *pathFilter = nil

	// Nil filter should not exclude anything
	assert.False(t, pf.shouldExclude("/health"))
	assert.False(t, pf.shouldExclude("/api/users"))
}

func TestMiddlewareOption_WithExcludePaths(t *testing.T) {
	t.Parallel()

	cfg := newMiddlewareConfig()
	WithExcludePaths("/health", "/metrics")(cfg)

	assert.True(t, cfg.pathFilter.shouldExclude("/health"))
	assert.True(t, cfg.pathFilter.shouldExclude("/metrics"))
	assert.False(t, cfg.pathFilter.shouldExclude("/api/users"))
}

func TestMiddlewareOption_WithExcludePrefixes(t *testing.T) {
	t.Parallel()

	cfg := newMiddlewareConfig()
	WithExcludePrefixes("/debug/", "/internal/")(cfg)

	assert.True(t, cfg.pathFilter.shouldExclude("/debug/pprof"))
	assert.True(t, cfg.pathFilter.shouldExclude("/internal/status"))
	assert.False(t, cfg.pathFilter.shouldExclude("/api/users"))
}

func TestMiddlewareOption_WithExcludePatterns(t *testing.T) {
	t.Parallel()

	cfg := newMiddlewareConfig()
	WithExcludePatterns(`^/v[0-9]+/internal/.*`, `^/admin/.*`)(cfg)

	assert.True(t, cfg.pathFilter.shouldExclude("/v1/internal/status"))
	assert.True(t, cfg.pathFilter.shouldExclude("/admin/users"))
	assert.False(t, cfg.pathFilter.shouldExclude("/api/users"))
}

func TestMiddlewareOption_WithExcludePatterns_InvalidPattern(t *testing.T) {
	t.Parallel()

	cfg := newMiddlewareConfig()
	// Invalid patterns are silently ignored
	WithExcludePatterns("[invalid-regex", `^/valid/.*`)(cfg)

	// Valid pattern should still work
	assert.True(t, cfg.pathFilter.shouldExclude("/valid/path"))
	// Invalid pattern didn't break anything
	assert.False(t, cfg.pathFilter.shouldExclude("/api/users"))
}
