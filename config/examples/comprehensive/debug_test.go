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
	"fmt"
	"os"
	"testing"

	"rivaas.dev/config"
	"rivaas.dev/config/source"
)

func TestDebugEnvVars(t *testing.T) {
	// Set a few environment variables
	os.Setenv("WEBAPP_SERVER_HOST", "test-host")
	os.Setenv("WEBAPP_SERVER_PORT", "8080")
	os.Setenv("WEBAPP_DATABASE_PRIMARY_HOST", "test-db")

	defer func() {
		os.Unsetenv("WEBAPP_SERVER_HOST")
		os.Unsetenv("WEBAPP_SERVER_PORT")
		os.Unsetenv("WEBAPP_DATABASE_PRIMARY_HOST")
	}()

	// Create environment variable source directly
	envSource := source.NewOSEnvVar("WEBAPP_")

	// Load configuration
	configMap, err := envSource.Load(context.Background())
	if err != nil {
		t.Fatalf("Failed to load environment variables: %v", err)
	}

	fmt.Printf("Loaded config: %+v\n", configMap)

	// Check specific values
	if host, ok := configMap["server"].(map[string]any); ok {
		if serverHost, exists := host["host"]; exists {
			fmt.Printf("Server host: %v\n", serverHost)
		} else {
			fmt.Printf("Server host not found in: %+v\n", host)
		}
	} else {
		fmt.Printf("Server section not found or not a map: %T %+v\n", configMap["server"], configMap["server"])
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

	fmt.Printf("Config server host: %s\n", cfg.String("server.host"))
	fmt.Printf("Config server port: %d\n", cfg.Int("server.port"))
	fmt.Printf("Config database host: %s\n", cfg.String("database.primary.host"))
}
