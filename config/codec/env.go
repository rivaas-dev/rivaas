// Copyright 2025 The Rivaas Authors
// Copyright 2025 Company.info B.V.
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

// Package codec provides functionality for encoding and decoding data.
package codec

import (
	"bytes"
	"fmt"
	"strings"
)

// TypeEnvVar is a constant representing the type of an environment variable codec.
const TypeEnvVar Type = "env_var"

// init registers the EnvVarCodec with the codec package under the TypeEnvVar type.
func init() {
	RegisterEncoder(TypeEnvVar, EnvVarCodec{})
	RegisterDecoder(TypeEnvVar, EnvVarCodec{})
}

// EnvVarCodec is a struct that implements the Codec interface for decoding environment variables.
type EnvVarCodec struct{}

// Encode encodes the provided value to environment variable format.
// This method is provided for interface compatibility but environment variables are typically read-only.
func (EnvVarCodec) Encode(_ any) ([]byte, error) {
	// Environment variables are typically read-only, so encoding is not supported
	return nil, fmt.Errorf("encoding to environment variables is not supported")
}

// Decode decodes the provided data bytes into a configuration map.
// The data is expected to be in the format of environment variables, with each line containing a key-value pair separated by an equals sign.
func (EnvVarCodec) Decode(data []byte, v any) error {
	conf := make(map[string]any)

	for _, env := range bytes.Split(data, []byte("\n")) {
		pair := strings.SplitN(string(env), "=", 2)
		if len(pair) != 2 {
			continue
		}

		// Sanitize key - trim whitespace
		key := strings.TrimSpace(pair[0])
		if key == "" {
			continue
		}

		// Split key by underscores and filter out empty parts
		rawParts := strings.Split(strings.ToLower(key), "_")
		parts := make([]string, 0, len(rawParts))
		for _, part := range rawParts {
			if part != "" {
				parts = append(parts, part)
			}
		}

		// Skip if no valid parts remain
		if len(parts) == 0 {
			continue
		}

		current := conf
		// Create nested structure for all parts except the last one
		for i := 0; i < len(parts)-1; i++ {
			part := parts[i]
			if _, exists := current[part]; !exists {
				current[part] = make(map[string]any)
			}
			// Handle type conflicts: if current[part] is not a map, overwrite it
			if nextMap, ok := current[part].(map[string]any); ok {
				current = nextMap
			} else {
				current[part] = make(map[string]any)
				if nextMap, ok := current[part].(map[string]any); ok {
					current = nextMap
				} else {
					// This should never happen, but handle it gracefully
					return fmt.Errorf("failed to create nested map for key: %s", part)
				}
			}
		}

		// Set the final value, trimming whitespace from value as well
		current[parts[len(parts)-1]] = strings.TrimSpace(pair[1])
	}

	ptr, ok := v.(*map[string]any)
	if !ok {
		return fmt.Errorf("EnvVarCodec.Decode: expected *map[string]any, got %T", v)
	}
	*ptr = conf

	return nil
}
