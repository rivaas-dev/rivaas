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

package main

import (
	"context"
	"testing"

	"rivaas.dev/config"
	"rivaas.dev/config/source"
)

func TestDebugEnvVars(t *testing.T) {
	// Set a few environment variables
	t.Setenv("WEBAPP_SERVER_HOST", "test-host")
	t.Setenv("WEBAPP_SERVER_PORT", "8080")
	t.Setenv("WEBAPP_DATABASE_PRIMARY_HOST", "test-db")

	// Create environment variable source directly
	envSource := source.NewOSEnvVar("WEBAPP_")

	// Load configuration
	configMap, err := envSource.Load(context.Background())
	if err != nil {
		t.Fatalf("Failed to load environment variables: %v", err)
	}

	t.Logf("Loaded config: %+v\n", configMap)

	// Check specific values
	if host, ok := configMap["server"].(map[string]any); ok {
		if serverHost, exists := host["host"]; exists {
			t.Logf("Server host: %v\n", serverHost)
		} else {
			t.Logf("Server host not found in: %+v\n", host)
		}
	} else {
		t.Logf("Server section not found or not a map: %T %+v\n", configMap["server"], configMap["server"])
	}

	// Test with config
	cfg, err := config.New(
		config.WithOSEnvVarSource("WEBAPP_"),
	)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	err = cfg.Load(context.Background())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	t.Logf("Config server host: %s\n", cfg.String("server.host"))
	t.Logf("Config server port: %d\n", cfg.Int("server.port"))
	t.Logf("Config database host: %s\n", cfg.String("database.primary.host"))
}
