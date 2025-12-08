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

// Package export provides OpenAPI specification export functionality.
// It supports exporting to OpenAPI 3.0.x and 3.1.x formats with proper
// down-leveling of 3.1-only features when targeting 3.0.
package export

import (
	"encoding/json"
	"maps"
	"strings"
)

// validateExtensionKey validates that an extension key starts with "x-".
// In 3.1.x, keys starting with "x-oai-" or "x-oas-" are reserved.
func validateExtensionKey(key, version string) error {
	if !strings.HasPrefix(key, "x-") {
		return &InvalidExtensionKeyError{Key: key}
	}
	// Check for reserved prefixes in 3.1.x
	if strings.HasPrefix(version, "3.1") && (strings.HasPrefix(key, "x-oai-") || strings.HasPrefix(key, "x-oas-")) {
		return &ReservedExtensionKeyError{Key: key}
	}

	return nil
}

// InvalidExtensionKeyError indicates an extension key doesn't start with "x-".
type InvalidExtensionKeyError struct {
	Key string
}

func (e *InvalidExtensionKeyError) Error() string {
	return "extension key must start with 'x-': " + e.Key
}

// Unwrap returns nil as InvalidExtensionKeyError is a leaf error type.
// This allows errors.Is() and errors.As() to work correctly.
func (e *InvalidExtensionKeyError) Unwrap() error {
	return nil
}

// ReservedExtensionKeyError indicates an extension key uses a reserved prefix.
type ReservedExtensionKeyError struct {
	Key string
}

func (e *ReservedExtensionKeyError) Error() string {
	return "extension key uses reserved prefix (x-oai- or x-oas-): " + e.Key
}

// Unwrap returns nil as ReservedExtensionKeyError is a leaf error type.
// This allows errors.Is() and errors.As() to work correctly.
func (e *ReservedExtensionKeyError) Unwrap() error {
	return nil
}

// copyExtensions copies extensions from model to export type with validation.
//
// Invalid extension keys (those that don't start with "x-" or use reserved
// prefixes in 3.1.x) are silently filtered out rather than causing an error.
// This allows projection to proceed even if some extensions are invalid,
// though validation should ideally happen at the API level (e.g., in Config.Validate).
//
// Returns nil if the input map is empty or all keys are filtered out.
func copyExtensions(in map[string]any, version string) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if err := validateExtensionKey(k, version); err != nil {
			// Skip invalid keys - validation should happen at API level
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}

	return out
}

// marshalWithExtensions marshals a struct with extensions inlined.
// This is a helper for custom MarshalJSON implementations.
//
// IMPORTANT: When calling this function, the caller MUST use a type alias
// to avoid infinite recursion. For example:
//
//	func (s *MyStruct) MarshalJSON() ([]byte, error) {
//	    type myStruct MyStruct  // Type alias prevents recursion
//	    return marshalWithExtensions(myStruct(*s), s.Extensions)
//	}
//
// Without the type alias, json.Marshal would recursively call MarshalJSON
// on the same type, causing infinite recursion. The type alias creates a
// new type that doesn't have the MarshalJSON method, allowing standard
// JSON marshaling to proceed.
func marshalWithExtensions(v any, extensions map[string]any) ([]byte, error) {
	// Marshal the base struct
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	if len(extensions) == 0 {
		return data, nil
	}

	// Parse the JSON into a map
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	// Merge extensions into the map
	maps.Copy(m, extensions)

	// Marshal back to JSON
	return json.Marshal(m)
}
