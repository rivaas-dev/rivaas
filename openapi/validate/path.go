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

package validate

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Path validation errors
var (
	ErrPathEmpty              = errors.New("path cannot be empty")
	ErrPathNoLeadingSlash     = errors.New("path must start with '/'")
	ErrPathDuplicateParameter = errors.New("duplicate path parameter")
	ErrPathInvalidParameter   = errors.New("invalid path parameter format")
)

// validParameterNamePattern validates parameter names: ^[a-zA-Z0-9._-]+$
// Per OpenAPI spec, parameter names should be alphanumeric with dots, underscores, and hyphens
var validParameterNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ValidatePath validates a path pattern for use in OpenAPI operations.
//
// It accepts both router-style (:param) and OpenAPI-style ({param}) syntax for
// developer convenience. The internal builder will normalize these to OpenAPI format.
//
// Validation checks:
//   - Non-empty path
//   - Path starts with '/'
//   - Valid path parameter syntax (:param or {param})
//   - No duplicate path parameters
//   - Parameter names match [a-zA-Z0-9._-]+
//   - Properly paired braces in {param} syntax
//
// Returns an error if validation fails.
func ValidatePath(path string) error {
	if path == "" {
		return ErrPathEmpty
	}

	if !strings.HasPrefix(path, "/") {
		return ErrPathNoLeadingSlash
	}

	// Track seen parameters to detect duplicates
	params := make(map[string]bool)
	segments := strings.SplitSeq(path, "/")

	for seg := range segments {
		if seg == "" {
			continue
		}

		var paramName string

		// Check for :param syntax (router-style)
		if after, ok := strings.CutPrefix(seg, ":"); ok {
			paramName = after
			if paramName == "" {
				return fmt.Errorf("%w: empty parameter name in segment ':%s'", ErrPathInvalidParameter, seg)
			}

			// Validate parameter name format
			if !validParameterNamePattern.MatchString(paramName) {
				return fmt.Errorf("%w: parameter name '%s' must match pattern [a-zA-Z0-9._-]+", ErrPathInvalidParameter, paramName)
			}
		}

		// Check for {param} syntax (OpenAPI-style)
		if strings.Contains(seg, "{") || strings.Contains(seg, "}") {
			// Must have both opening and closing braces
			if !strings.HasPrefix(seg, "{") || !strings.HasSuffix(seg, "}") {
				return fmt.Errorf("%w: mismatched braces in segment '%s' (use ':param' or '{param}')", ErrPathInvalidParameter, seg)
			}

			paramName = strings.TrimPrefix(strings.TrimSuffix(seg, "}"), "{")
			if paramName == "" {
				return fmt.Errorf("%w: empty parameter name in segment '{}'", ErrPathInvalidParameter)
			}

			// Check for nested or malformed braces
			if strings.Contains(paramName, "{") || strings.Contains(paramName, "}") {
				return fmt.Errorf("%w: parameter name cannot contain braces: '%s'", ErrPathInvalidParameter, seg)
			}

			// Check for slash inside parameter (e.g. {user/id})
			if strings.Contains(paramName, "/") {
				return fmt.Errorf("%w: parameter name cannot contain '/': '%s'", ErrPathInvalidParameter, paramName)
			}

			// Validate parameter name format
			if !validParameterNamePattern.MatchString(paramName) {
				return fmt.Errorf("%w: parameter name '%s' must match pattern [a-zA-Z0-9._-]+", ErrPathInvalidParameter, paramName)
			}
		}

		// Check for duplicate parameters
		if paramName != "" {
			if params[paramName] {
				return fmt.Errorf("%w: '%s' appears multiple times", ErrPathDuplicateParameter, paramName)
			}
			params[paramName] = true
		}
	}

	return nil
}
