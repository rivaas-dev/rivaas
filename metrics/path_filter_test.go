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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPathFilter_ExactPaths(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithExcludePaths("/health", "/metrics", "/ready"),
	)
	defer config.Shutdown(nil)

	assert.True(t, config.ShouldExcludePath("/health"))
	assert.True(t, config.ShouldExcludePath("/metrics"))
	assert.True(t, config.ShouldExcludePath("/ready"))
	assert.False(t, config.ShouldExcludePath("/api/users"))
	assert.False(t, config.ShouldExcludePath("/health/check")) // Not exact match
}

func TestPathFilter_Prefixes(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithExcludePrefixes("/debug/", "/internal/"),
	)
	defer config.Shutdown(nil)

	assert.True(t, config.ShouldExcludePath("/debug/pprof"))
	assert.True(t, config.ShouldExcludePath("/debug/vars"))
	assert.True(t, config.ShouldExcludePath("/internal/status"))
	assert.True(t, config.ShouldExcludePath("/internal/config/reload"))
	assert.False(t, config.ShouldExcludePath("/api/users"))
	assert.False(t, config.ShouldExcludePath("/debugger")) // Doesn't start with /debug/
}

func TestPathFilter_Patterns(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithExcludePatterns(`^/v[0-9]+/internal/.*`, `^/admin/.*`),
	)
	defer config.Shutdown(nil)

	assert.True(t, config.ShouldExcludePath("/v1/internal/status"))
	assert.True(t, config.ShouldExcludePath("/v2/internal/config"))
	assert.True(t, config.ShouldExcludePath("/admin/users"))
	assert.True(t, config.ShouldExcludePath("/admin/settings/global"))
	assert.False(t, config.ShouldExcludePath("/api/users"))
	assert.False(t, config.ShouldExcludePath("/v1/users")) // Doesn't match internal
}

func TestPathFilter_Combined(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
		WithExcludePaths("/health", "/ready"),
		WithExcludePrefixes("/debug/"),
		WithExcludePatterns(`^/v[0-9]+/internal/.*`),
	)
	defer config.Shutdown(nil)

	// Exact paths
	assert.True(t, config.ShouldExcludePath("/health"))
	assert.True(t, config.ShouldExcludePath("/ready"))

	// Prefixes
	assert.True(t, config.ShouldExcludePath("/debug/pprof"))

	// Patterns
	assert.True(t, config.ShouldExcludePath("/v1/internal/status"))

	// Not excluded
	assert.False(t, config.ShouldExcludePath("/api/users"))
}

func TestPathFilter_InvalidPattern(t *testing.T) {
	t.Parallel()

	_, err := New(
		WithServiceName("test-service"),
		WithExcludePatterns("[invalid-regex"),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex pattern")
}

func TestPathFilter_EmptyConfig(t *testing.T) {
	t.Parallel()

	config := MustNew(
		WithServiceName("test-service"),
	)
	defer config.Shutdown(nil)

	// With no exclusions configured, nothing should be excluded
	assert.False(t, config.ShouldExcludePath("/health"))
	assert.False(t, config.ShouldExcludePath("/api/users"))
}
