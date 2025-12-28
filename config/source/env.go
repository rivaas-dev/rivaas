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

package source

import (
	"context"
	"fmt"
	"os"
	"strings"

	"rivaas.dev/config/codec"
)

// OSEnvVar represents a configuration source that loads data from environment variables.
// It filters environment variables by prefix and creates nested configuration structures
// based on underscore-separated variable names.
//
// For example, with prefix "APP_", the environment variable "APP_SERVER_PORT" becomes
// the configuration key "server.port".
type OSEnvVar struct {
	prefix  string
	decoder codec.Decoder
}

// NewOSEnvVar creates a new OSEnvVar source with the specified prefix.
// Only environment variables starting with this prefix will be loaded.
// The prefix is stripped from variable names before processing.
func NewOSEnvVar(prefix string) *OSEnvVar {
	return &OSEnvVar{
		prefix:  prefix,
		decoder: codec.EnvVarCodec{},
	}
}

// Load reads environment variables with the configured prefix and decodes them into a map[string]any.
// Variable names are converted to lowercase and underscores create nested structures.
//
// Example:
//
//	APP_SERVER_PORT=8080     -> server.port = "8080"
//	APP_SERVER_HOST=localhost -> server.host = "localhost"
//	APP_DEBUG=true           -> debug = "true"
//
// Errors:
//   - Returns error if decoding fails
func (e *OSEnvVar) Load(_ context.Context) (map[string]any, error) {
	validEnv := make([]string, 0, len(os.Environ()))

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, e.prefix) {
			continue
		}

		validEnv = append(validEnv, strings.TrimPrefix(env, e.prefix))
	}

	data := strings.Join(validEnv, "\n")

	var config map[string]any
	if err := e.decoder.Decode([]byte(data), &config); err != nil {
		return nil, fmt.Errorf("failed to decode environment variables: %w", err)
	}

	return config, nil
}
