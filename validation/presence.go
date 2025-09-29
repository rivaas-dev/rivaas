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

package validation

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PresenceMap tracks which fields are present in the request body.
// Keys are normalized dot paths (e.g., "items.2.price"), values are booleans.
//
// PresenceMap is used for partial update validation (PATCH requests),
// where only present fields should be validated, while absent fields
// should be ignored even if they have "required" constraints.
type PresenceMap map[string]bool

// Has returns true if the exact path is present.
func (pm PresenceMap) Has(path string) bool {
	return pm != nil && pm[path]
}

// HasPrefix returns true if any path with the given prefix is present.
// HasPrefix is useful for checking if a nested object or array element is present.
func (pm PresenceMap) HasPrefix(prefix string) bool {
	if pm == nil {
		return false
	}
	prefixDot := prefix + "."
	for path := range pm {
		if path == prefix || strings.HasPrefix(path, prefixDot) {
			return true
		}
	}
	return false
}

// LeafPaths returns paths that aren't prefixes of others.
// LeafPaths is useful for partial validation where only leaf fields that were actually provided
// should be validated, not their parent objects.
//
// Example:
//   - If presence contains "address" and "address.city", only "address.city" is a leaf.
//   - If presence contains "items.0" and "items.0.name", only "items.0.name" is a leaf.
func (pm PresenceMap) LeafPaths() []string {
	if pm == nil {
		return nil
	}

	paths := make([]string, 0, len(pm))
	for p := range pm {
		paths = append(paths, p)
	}

	// Sort to process in order
	sort.Strings(paths)

	// Use single-pass algorithm: if next path has current as prefix, current is not a leaf
	isLeaf := make([]bool, len(paths))
	for i := range isLeaf {
		isLeaf[i] = true
	}

	for i := 0; i < len(paths)-1; i++ {
		// If next path has current as prefix, current is not a leaf
		if strings.HasPrefix(paths[i+1], paths[i]+".") {
			isLeaf[i] = false
		}
	}

	leaves := make([]string, 0, len(paths))
	for i, leaf := range isLeaf {
		if leaf {
			leaves = append(leaves, paths[i])
		}
	}

	return leaves
}

// ComputePresence analyzes raw JSON and returns a map of present field paths.
// ComputePresence enables partial validation where only provided fields are validated.
//
// Example:
//
//	rawJSON := []byte(`{"user": {"name": "Alice", "age": 0}}`)
//	presence, err := ComputePresence(rawJSON)
//	// Returns: {"user": true, "user.name": true, "user.age": true}
//
// ComputePresence has a maximum recursion depth of 100 to prevent stack overflow
// from deeply nested JSON structures.
func ComputePresence(rawJSON []byte) (PresenceMap, error) {
	if len(rawJSON) == 0 {
		return nil, nil
	}

	var data map[string]any
	if err := json.Unmarshal(rawJSON, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON for presence tracking: %w", err)
	}

	pm := make(PresenceMap)
	markPresence(data, "", pm, 0)
	return pm, nil
}

// markPresence recursively marks fields as present in the PresenceMap.
// depth tracks recursion depth to prevent stack overflow from malicious input.
func markPresence(m map[string]any, prefix string, pm PresenceMap, depth int) {
	if depth > maxRecursionDepth {
		return // Prevent stack overflow from deeply nested structures
	}

	for k, v := range m {
		//nolint:copyloopvar // path is modified conditionally
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		pm[path] = true

		if nested, ok := v.(map[string]any); ok {
			markPresence(nested, path, pm, depth+1)
		}

		if arr, ok := v.([]any); ok {
			for i, item := range arr {
				itemPath := path + "." + strconv.Itoa(i)
				pm[itemPath] = true
				if nestedMap, ok := item.(map[string]any); ok {
					markPresence(nestedMap, itemPath, pm, depth+1)
				}
			}
		}
	}
}
