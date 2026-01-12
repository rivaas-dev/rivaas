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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/config"
	"rivaas.dev/config/codec"
)

func TestMixedYAMLAndEnvironmentVariables(t *testing.T) {
	// Set up test environment variables to override YAML defaults
	t.Setenv("WEBAPP_SERVER_PORT", "9090")
	t.Setenv("WEBAPP_DATABASE_PRIMARY_HOST", "test-db")
	t.Setenv("WEBAPP_AUTH_JWT_SECRET", "test-secret")
	t.Setenv("WEBAPP_FEATURES_DEBUG_MODE", "false")

	// Create configuration with both YAML and environment variables
	cfg, err := config.New(
		config.WithFileSource("config.yaml", codec.TypeYAML),
		config.WithOSEnvVarSource("WEBAPP_"),
	)
	require.NoError(t, err)

	// Load configuration
	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// Test that environment variables override YAML defaults
	assert.Equal(t, "localhost", cfg.String("server.host"))         // From YAML (not overridden)
	assert.Equal(t, 9090, cfg.Int("server.port"))                   // From env (overrides YAML's 3000)
	assert.Equal(t, "test-db", cfg.String("database.primary.host")) // From env (overrides YAML's localhost)
	assert.Equal(t, 5432, cfg.Int("database.primary.port"))         // From YAML (not overridden)
	assert.Equal(t, "test-secret", cfg.String("auth.jwt.secret"))   // From env (overrides YAML's dev secret)
	assert.False(t, cfg.Bool("features.debug.mode"))                // From env (overrides YAML's true)

	// Test struct binding
	var wc WebAppConfig
	cfgWithBinding, err := config.New(
		config.WithFileSource("config.yaml", codec.TypeYAML),
		config.WithOSEnvVarSource("WEBAPP_"),
		config.WithBinding(&wc),
	)
	require.NoError(t, err)

	err = cfgWithBinding.Load(context.Background())
	require.NoError(t, err)

	// Verify struct binding reflects the mixed configuration
	assert.Equal(t, "localhost", wc.Server.Host)         // From YAML
	assert.Equal(t, 9090, wc.Server.Port)                // From env
	assert.Equal(t, "test-db", wc.Database.Primary.Host) // From env
	assert.Equal(t, 5432, wc.Database.Primary.Port)      // From YAML
	assert.Equal(t, "test-secret", wc.Auth.JWT.Secret)   // From env
	assert.False(t, wc.Features.Debug.Mode)              // From env
}

func TestYAMLOnlyConfiguration(t *testing.T) {
	// Create configuration with only YAML file
	cfg, err := config.New(
		config.WithFileSource("config.yaml", codec.TypeYAML),
	)
	require.NoError(t, err)

	// Load configuration
	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// Test that YAML defaults are used
	assert.Equal(t, "localhost", cfg.String("server.host"))
	assert.Equal(t, 3000, cfg.Int("server.port"))                     // YAML default
	assert.Equal(t, "localhost", cfg.String("database.primary.host")) // YAML default
	assert.Equal(t, 5432, cfg.Int("database.primary.port"))
	assert.Equal(t, "dev-jwt-secret-change-in-production", cfg.String("auth.jwt.secret")) // YAML default
	assert.True(t, cfg.Bool("features.debug.mode"))                                       // YAML default
}

func TestEnvironmentVariablesOnly(t *testing.T) {
	// Set environment variables
	t.Setenv("WEBAPP_SERVER_HOST", "env-host")
	t.Setenv("WEBAPP_SERVER_PORT", "8080")
	t.Setenv("WEBAPP_DATABASE_PRIMARY_HOST", "env-db")
	t.Setenv("WEBAPP_AUTH_JWT_SECRET", "env-secret")

	// Create configuration with only environment variables
	cfg, err := config.New(
		config.WithOSEnvVarSource("WEBAPP_"),
	)
	require.NoError(t, err)

	// Load configuration
	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// Test that environment variables are used
	assert.Equal(t, "env-host", cfg.String("server.host"))
	assert.Equal(t, 8080, cfg.Int("server.port"))
	assert.Equal(t, "env-db", cfg.String("database.primary.host"))
	assert.Equal(t, "env-secret", cfg.String("auth.jwt.secret"))
}
