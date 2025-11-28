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
	"fmt"
	"regexp"
	"strings"
)

// pathFilter handles path exclusion logic for metrics.
// It supports exact paths, prefixes, and regex patterns.
type pathFilter struct {
	paths    map[string]bool
	prefixes []string
	patterns []*regexp.Regexp
}

// newPathFilter creates a new path filter.
func newPathFilter() *pathFilter {
	return &pathFilter{
		paths: make(map[string]bool),
	}
}

// addPaths adds exact paths to exclude.
func (pf *pathFilter) addPaths(paths ...string) {
	for _, p := range paths {
		pf.paths[p] = true
	}
}

// addPrefixes adds path prefixes to exclude.
func (pf *pathFilter) addPrefixes(prefixes ...string) {
	pf.prefixes = append(pf.prefixes, prefixes...)
}

// addPatterns adds compiled regex patterns to exclude.
func (pf *pathFilter) addPatterns(patterns ...*regexp.Regexp) {
	pf.patterns = append(pf.patterns, patterns...)
}

// shouldExclude returns true if the path should be excluded from metrics.
func (pf *pathFilter) shouldExclude(path string) bool {
	if pf == nil {
		return false
	}

	// Check exact paths (O(1) lookup)
	if pf.paths[path] {
		return true
	}

	// Check prefixes
	for _, prefix := range pf.prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	// Check patterns
	for _, pattern := range pf.patterns {
		if pattern.MatchString(path) {
			return true
		}
	}

	return false
}

// WithExcludePrefixes excludes paths with the given prefixes from metrics collection.
// This is useful for excluding entire path hierarchies like /debug/, /internal/, etc.
//
// Example:
//
//	config := metrics.MustNew(
//	    metrics.WithExcludePrefixes("/debug/", "/internal/"),
//	)
func WithExcludePrefixes(prefixes ...string) Option {
	return func(c *Config) {
		if c.pathFilter == nil {
			c.pathFilter = newPathFilter()
		}
		c.pathFilter.addPrefixes(prefixes...)
	}
}

// WithExcludePatterns excludes paths matching the given regex patterns from metrics collection.
// The patterns are compiled once during configuration.
// Invalid regex patterns will cause New() to return an error.
//
// Example:
//
//	config := metrics.MustNew(
//	    metrics.WithExcludePatterns(`^/v[0-9]+/internal/.*`, `^/debug/.*`),
//	)
func WithExcludePatterns(patterns ...string) Option {
	return func(c *Config) {
		if c.pathFilter == nil {
			c.pathFilter = newPathFilter()
		}
		for _, pattern := range patterns {
			compiled, err := regexp.Compile(pattern)
			if err != nil {
				// Store error to be returned during validation
				if c.validationErrors == nil {
					c.validationErrors = make([]error, 0, 1)
				}
				c.validationErrors = append(c.validationErrors,
					fmt.Errorf("invalid regex pattern for path exclusion %q: %w", pattern, err))
				c.logError("Invalid regex pattern for path exclusion",
					"pattern", pattern,
					"error", err,
				)
				continue
			}
			c.pathFilter.addPatterns(compiled)
		}
	}
}

// ShouldExcludePath returns true if the given path should be excluded from metrics.
// Checks exact paths, prefixes, and regex patterns.
func (c *Config) ShouldExcludePath(path string) bool {
	if c.pathFilter == nil {
		return false
	}
	return c.pathFilter.shouldExclude(path)
}
