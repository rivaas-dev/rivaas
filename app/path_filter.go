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

package app

import (
	"regexp"
	"strings"
)

// pathFilter handles path exclusion logic for unified observability.
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

// newPathFilterWithDefaults creates a path filter with default health/probe paths.
func newPathFilterWithDefaults() *pathFilter {
	pf := newPathFilter()
	// Default paths to exclude from observability
	pf.addPaths(
		"/health", "/healthz",
		"/ready", "/readyz",
		"/live", "/livez",
		"/metrics",
	)
	// Default prefixes to exclude
	pf.addPrefixes("/debug/")
	return pf
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

// shouldExclude returns true if the path should be excluded from observability.
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
