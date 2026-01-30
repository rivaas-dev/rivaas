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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/config"
)

func TestWebAppConfig_EnvironmentVariables(t *testing.T) {
	// Set up test environment variables (all required fields)
	t.Setenv("WEBAPP_SERVER_HOST", "test-host")
	t.Setenv("WEBAPP_SERVER_PORT", "9090")
	t.Setenv("WEBAPP_DATABASE_PRIMARY_HOST", "test-db")
	t.Setenv("WEBAPP_DATABASE_PRIMARY_PORT", "5432")
	t.Setenv("WEBAPP_DATABASE_PRIMARY_DATABASE", "testdb")
	t.Setenv("WEBAPP_AUTH_JWT_SECRET", "test-secret")
	t.Setenv("WEBAPP_AUTH_TOKEN_DURATION", "1h")
	t.Setenv("WEBAPP_FEATURES_DEBUG_MODE", "true")

	// Debug: Check if environment variables are set
	t.Logf("WEBAPP_SERVER_HOST: %s\n", os.Getenv("WEBAPP_SERVER_HOST"))
	t.Logf("WEBAPP_AUTH_JWT_SECRET: %s\n", os.Getenv("WEBAPP_AUTH_JWT_SECRET"))

	// Create configuration without binding to test direct access
	cfg, err := config.New(
		config.WithEnv("WEBAPP_"),
	)
	require.NoError(t, err)

	// Load configuration
	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// Debug: Check what was loaded
	t.Logf("Loaded server.host: %s\n", cfg.String("server.host"))
	t.Logf("Loaded auth.jwt.secret: %s\n", cfg.String("auth.jwt.secret"))
	t.Logf("Loaded features.debug.mode: %t\n", cfg.Bool("features.debug.mode"))

	// Test direct configuration access
	assert.Equal(t, "test-host", cfg.String("server.host"))
	assert.Equal(t, 9090, cfg.Int("server.port"))
	assert.Equal(t, "test-db", cfg.String("database.primary.host"))
	assert.Equal(t, 5432, cfg.Int("database.primary.port"))
	assert.Equal(t, "testdb", cfg.String("database.primary.database"))
	assert.Equal(t, "test-secret", cfg.String("auth.jwt.secret"))
	assert.True(t, cfg.Bool("features.debug.mode"))

	// Now test with binding
	var wc WebAppConfig
	cfgWithBinding, err := config.New(
		config.WithEnv("WEBAPP_"),
		config.WithBinding(&wc),
	)
	require.NoError(t, err)

	// Load configuration with binding
	err = cfgWithBinding.Load(context.Background())
	require.NoError(t, err)

	// Test struct binding
	assert.Equal(t, "test-host", wc.Server.Host)
	assert.Equal(t, 9090, wc.Server.Port)
	assert.Equal(t, "test-db", wc.Database.Primary.Host)
	assert.Equal(t, 5432, wc.Database.Primary.Port)
	assert.Equal(t, "testdb", wc.Database.Primary.Database)
	assert.Equal(t, "test-secret", wc.Auth.JWT.Secret)
	assert.True(t, wc.Features.Debug.Mode)
}

func TestWebAppConfig_NestedStructures(t *testing.T) {
	// Test nested environment variable mapping (including required fields)
	t.Setenv("WEBAPP_SERVER_HOST", "test-host")
	t.Setenv("WEBAPP_SERVER_PORT", "9090")
	t.Setenv("WEBAPP_DATABASE_PRIMARY_HOST", "test-db")
	t.Setenv("WEBAPP_DATABASE_PRIMARY_PORT", "5432")
	t.Setenv("WEBAPP_DATABASE_PRIMARY_DATABASE", "testdb")
	t.Setenv("WEBAPP_AUTH_JWT_SECRET", "test-secret")
	t.Setenv("WEBAPP_AUTH_TOKEN_DURATION", "1h")
	t.Setenv("WEBAPP_SERVER_TLS_ENABLED", "true")
	t.Setenv("WEBAPP_SERVER_TLS_CERT_FILE", "/path/to/cert.pem")
	t.Setenv("WEBAPP_SERVER_TLS_KEY_FILE", "/path/to/key.pem")
	t.Setenv("WEBAPP_DATABASE_POOL_MAX_OPEN", "50")
	t.Setenv("WEBAPP_DATABASE_POOL_MAX_IDLE", "10")

	// Test direct access first
	cfg, err := config.New(
		config.WithEnv("WEBAPP_"),
	)
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// Test direct access to nested values
	assert.True(t, cfg.Bool("server.tls.enabled"))
	assert.Equal(t, "/path/to/cert.pem", cfg.String("server.tls.cert.file"))
	assert.Equal(t, "/path/to/key.pem", cfg.String("server.tls.key.file"))
	assert.Equal(t, 50, cfg.Int("database.pool.max.open"))
	assert.Equal(t, 10, cfg.Int("database.pool.max.idle"))

	// Now test with binding
	var wc WebAppConfig
	cfgWithBinding, err := config.New(
		config.WithEnv("WEBAPP_"),
		config.WithBinding(&wc),
	)
	require.NoError(t, err)

	err = cfgWithBinding.Load(context.Background())
	require.NoError(t, err)

	// Test nested TLS configuration
	assert.True(t, wc.Server.TLS.Enabled)
	assert.Equal(t, "/path/to/cert.pem", wc.Server.TLS.Cert.File)
	assert.Equal(t, "/path/to/key.pem", wc.Server.TLS.Key.File)

	// Test nested database pool configuration
	assert.Equal(t, 50, wc.Database.Pool.Max.Open)
	assert.Equal(t, 10, wc.Database.Pool.Max.Idle)
}
