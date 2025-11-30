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
)

// FuzzMetricNameValidation tests metric name validation with random inputs.
// This helps discover edge cases that table-driven tests might miss.
//
// Run with: go test -fuzz=FuzzMetricNameValidation -fuzztime=30s ./metrics
func FuzzMetricNameValidation(f *testing.F) {
	// Seed corpus with known good inputs
	f.Add("valid_metric")
	f.Add("my.metric.name")
	f.Add("metric123")
	f.Add("MyMetric")
	f.Add("a")
	f.Add("my-metric-name")
	f.Add("my_metric.name-v2")

	// Seed corpus with known bad inputs
	f.Add("")
	f.Add("__reserved")
	f.Add("http_prefix")
	f.Add("router_metric")
	f.Add("123invalid")
	f.Add("_underscore_start")
	f.Add("metric@invalid#chars")
	f.Add("metric with spaces")
	f.Add("metric\ttab")
	f.Add("metric\nnewline")

	// Very long name
	longName := make([]byte, 300)
	for i := range longName {
		longName[i] = 'a'
	}
	f.Add(string(longName))

	f.Fuzz(func(t *testing.T, name string) {
		// The function should never panic
		err := validateMetricName(name)

		// Verify consistency: if validation passes, certain invariants must hold
		if err == nil {
			// Valid names must be non-empty
			if len(name) == 0 {
				t.Errorf("validateMetricName returned nil error for empty name")
			}

			// Valid names must not exceed max length
			if len(name) > maxMetricNameLength {
				t.Errorf("validateMetricName returned nil error for name exceeding max length: %d", len(name))
			}

			// Valid names must match the regex
			if !metricNameRegex.MatchString(name) {
				t.Errorf("validateMetricName returned nil error for name that doesn't match regex: %q", name)
			}

			// Valid names must not start with reserved prefixes
			for _, prefix := range reservedPrefixes {
				if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
					t.Errorf("validateMetricName returned nil error for name with reserved prefix %q: %q", prefix, name)
				}
			}
		}
	})
}

// FuzzPathFilter tests path filtering with random paths.
// This ensures the path filter doesn't crash on unexpected input.
//
// Run with: go test -fuzz=FuzzPathFilter -fuzztime=30s ./metrics
func FuzzPathFilter(f *testing.F) {
	// Seed corpus with typical paths
	f.Add("/health")
	f.Add("/metrics")
	f.Add("/api/users")
	f.Add("/api/users/123")
	f.Add("/debug/pprof")
	f.Add("/debug/vars")
	f.Add("/v1/internal/status")
	f.Add("/admin/users")

	// Edge cases
	f.Add("")
	f.Add("/")
	f.Add("//")
	f.Add("///")
	f.Add("/path/with spaces")
	f.Add("/path\twith\ttabs")
	f.Add("/path\nwith\nnewlines")
	f.Add("not-starting-with-slash")
	f.Add("/very/long/path/that/goes/on/and/on/and/on/and/on")

	// Special characters
	f.Add("/path?query=value")
	f.Add("/path#fragment")
	f.Add("/path%20encoded")
	f.Add("/path/../traversal")
	f.Add("/path/./current")

	f.Fuzz(func(t *testing.T, path string) {
		// Create a path filter with various exclusion rules
		pf := newPathFilter()
		pf.addPaths("/health", "/metrics", "/ready")
		pf.addPrefixes("/debug/", "/internal/")

		// Add a regex pattern
		pattern, err := regexp.Compile(`^/v[0-9]+/internal/.*`)
		if err != nil {
			t.Fatalf("failed to compile pattern: %v", err)
		}
		pf.addPatterns(pattern)

		// The function should never panic
		result := pf.shouldExclude(path)

		// Verify consistency: exact matches should always be excluded
		if path == "/health" || path == "/metrics" || path == "/ready" {
			if !result {
				t.Errorf("shouldExclude returned false for exact match path: %q", path)
			}
		}
	})
}

// FuzzPathFilterNil tests that nil path filter handles all inputs safely.
//
// Run with: go test -fuzz=FuzzPathFilterNil -fuzztime=10s ./metrics
func FuzzPathFilterNil(f *testing.F) {
	f.Add("/health")
	f.Add("")
	f.Add("/any/path")

	f.Fuzz(func(t *testing.T, path string) {
		var pf *pathFilter = nil

		// Nil filter should never panic and always return false
		result := pf.shouldExclude(path)
		if result {
			t.Errorf("nil pathFilter.shouldExclude returned true for path: %q", path)
		}
	})
}

// FuzzPathFilterEmpty tests that empty path filter handles all inputs safely.
//
// Run with: go test -fuzz=FuzzPathFilterEmpty -fuzztime=10s ./metrics
func FuzzPathFilterEmpty(f *testing.F) {
	f.Add("/health")
	f.Add("")
	f.Add("/any/path")

	f.Fuzz(func(t *testing.T, path string) {
		pf := newPathFilter()

		// Empty filter should never panic and always return false
		result := pf.shouldExclude(path)
		if result {
			t.Errorf("empty pathFilter.shouldExclude returned true for path: %q", path)
		}
	})
}
