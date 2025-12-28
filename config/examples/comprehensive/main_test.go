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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/config"
)

func TestWebAppConfig_EnvironmentVariables(t *testing.T) {
	// Set up test environment variables (all required fields)
	os.Setenv("WEBAPP_SERVER_HOST", "test-host")
	os.Setenv("WEBAPP_SERVER_PORT", "9090")
	os.Setenv("WEBAPP_DATABASE_PRIMARY_HOST", "test-db")
	os.Setenv("WEBAPP_DATABASE_PRIMARY_PORT", "5432")
	os.Setenv("WEBAPP_DATABASE_PRIMARY_DATABASE", "testdb")
	os.Setenv("WEBAPP_AUTH_JWT_SECRET", "test-secret")
	os.Setenv("WEBAPP_AUTH_TOKEN_DURATION", "1h")
	os.Setenv("WEBAPP_FEATURES_DEBUG_MODE", "true")

	// Debug: Check if environment variables are set
	fmt.Printf("WEBAPP_SERVER_HOST: %s\n", os.Getenv("WEBAPP_SERVER_HOST"))
	fmt.Printf("WEBAPP_AUTH_JWT_SECRET: %s\n", os.Getenv("WEBAPP_AUTH_JWT_SECRET"))

	// Clean up environment variables after test
	defer func() {
		os.Unsetenv("WEBAPP_SERVER_HOST")
		os.Unsetenv("WEBAPP_SERVER_PORT")
		os.Unsetenv("WEBAPP_DATABASE_PRIMARY_HOST")
		os.Unsetenv("WEBAPP_DATABASE_PRIMARY_PORT")
		os.Unsetenv("WEBAPP_DATABASE_PRIMARY_DATABASE")
		os.Unsetenv("WEBAPP_AUTH_JWT_SECRET")
		os.Unsetenv("WEBAPP_AUTH_TOKEN_DURATION")
		os.Unsetenv("WEBAPP_FEATURES_DEBUG_MODE")
	}()

	// Create configuration without binding to test direct access
	cfg, err := config.New(
		config.WithOSEnvVarSource("WEBAPP_"),
	)
	require.NoError(t, err)

	// Load configuration
	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// Debug: Check what was loaded
	fmt.Printf("Loaded server.host: %s\n", cfg.String("server.host"))
	fmt.Printf("Loaded auth.jwt.secret: %s\n", cfg.String("auth.jwt.secret"))
	fmt.Printf("Loaded features.debug.mode: %t\n", cfg.Bool("features.debug.mode"))

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
		config.WithOSEnvVarSource("WEBAPP_"),
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
	os.Setenv("WEBAPP_SERVER_HOST", "test-host")
	os.Setenv("WEBAPP_SERVER_PORT", "9090")
	os.Setenv("WEBAPP_DATABASE_PRIMARY_HOST", "test-db")
	os.Setenv("WEBAPP_DATABASE_PRIMARY_PORT", "5432")
	os.Setenv("WEBAPP_DATABASE_PRIMARY_DATABASE", "testdb")
	os.Setenv("WEBAPP_AUTH_JWT_SECRET", "test-secret")
	os.Setenv("WEBAPP_AUTH_TOKEN_DURATION", "1h")

	os.Setenv("WEBAPP_SERVER_TLS_ENABLED", "true")
	os.Setenv("WEBAPP_SERVER_TLS_CERT_FILE", "/path/to/cert.pem")
	os.Setenv("WEBAPP_SERVER_TLS_KEY_FILE", "/path/to/key.pem")
	os.Setenv("WEBAPP_DATABASE_POOL_MAX_OPEN", "50")
	os.Setenv("WEBAPP_DATABASE_POOL_MAX_IDLE", "10")

	defer func() {
		os.Unsetenv("WEBAPP_SERVER_HOST")
		os.Unsetenv("WEBAPP_SERVER_PORT")
		os.Unsetenv("WEBAPP_DATABASE_PRIMARY_HOST")
		os.Unsetenv("WEBAPP_DATABASE_PRIMARY_PORT")
		os.Unsetenv("WEBAPP_DATABASE_PRIMARY_DATABASE")
		os.Unsetenv("WEBAPP_AUTH_JWT_SECRET")
		os.Unsetenv("WEBAPP_AUTH_TOKEN_DURATION")
		os.Unsetenv("WEBAPP_SERVER_TLS_ENABLED")
		os.Unsetenv("WEBAPP_SERVER_TLS_CERT_FILE")
		os.Unsetenv("WEBAPP_SERVER_TLS_KEY_FILE")
		os.Unsetenv("WEBAPP_DATABASE_POOL_MAX_OPEN")
		os.Unsetenv("WEBAPP_DATABASE_POOL_MAX_IDLE")
	}()

	// Test direct access first
	cfg, err := config.New(
		config.WithOSEnvVarSource("WEBAPP_"),
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
		config.WithOSEnvVarSource("WEBAPP_"),
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
