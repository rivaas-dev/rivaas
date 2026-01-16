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

//go:build integration

package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/config"
	"rivaas.dev/config/codec"
)

// TestIntegration_FileSourceWithYAML tests end-to-end YAML file loading.
func TestIntegration_FileSourceWithYAML(t *testing.T) {
	t.Parallel()

	// Create temporary YAML file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := []byte(`
server:
  host: localhost
  port: 8080
  tls:
    enabled: true
    cert: /path/to/cert.pem
database:
  driver: postgres
  host: db.example.com
  port: 5432
  credentials:
    user: dbuser
    password: dbpass
logging:
  level: info
  format: json
`)

	err := os.WriteFile(configFile, yamlContent, 0o600)
	require.NoError(t, err)

	// Load configuration
	cfg, err := config.New(
		config.WithFileSource(configFile, codec.TypeYAML),
	)
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// Verify loaded values
	assert.Equal(t, "localhost", cfg.String("server.host"))
	assert.Equal(t, 8080, cfg.Int("server.port"))
	assert.True(t, cfg.Bool("server.tls.enabled"))
	assert.Equal(t, "/path/to/cert.pem", cfg.String("server.tls.cert"))
	assert.Equal(t, "postgres", cfg.String("database.driver"))
	assert.Equal(t, "db.example.com", cfg.String("database.host"))
	assert.Equal(t, 5432, cfg.Int("database.port"))
	assert.Equal(t, "dbuser", cfg.String("database.credentials.user"))
	assert.Equal(t, "info", cfg.String("logging.level"))
	assert.Equal(t, "json", cfg.String("logging.format"))
}

// TestIntegration_FileSourceWithJSON tests end-to-end JSON file loading.
func TestIntegration_FileSourceWithJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.json")

	jsonContent := []byte(`{
		"app": {
			"name": "MyApp",
			"version": "1.0.0",
			"features": ["auth", "api", "metrics"]
		},
		"cache": {
			"enabled": true,
			"ttl": 3600
		}
	}`)

	err := os.WriteFile(configFile, jsonContent, 0o600)
	require.NoError(t, err)

	cfg, err := config.New(
		config.WithFileSource(configFile, codec.TypeJSON),
	)
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "MyApp", cfg.String("app.name"))
	assert.Equal(t, "1.0.0", cfg.String("app.version"))
	assert.Equal(t, []string{"auth", "api", "metrics"}, cfg.StringSlice("app.features"))
	assert.True(t, cfg.Bool("cache.enabled"))
	assert.Equal(t, 3600, cfg.Int("cache.ttl"))
}

// TestIntegration_FileSourceWithTOML tests end-to-end TOML file loading.
func TestIntegration_FileSourceWithTOML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.toml")

	tomlContent := []byte(`
title = "My Application"

[server]
host = "0.0.0.0"
port = 9090

[database]
driver = "mysql"
max_connections = 100
`)

	err := os.WriteFile(configFile, tomlContent, 0o600)
	require.NoError(t, err)

	cfg, err := config.New(
		config.WithFileSource(configFile, codec.TypeTOML),
	)
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "My Application", cfg.String("title"))
	assert.Equal(t, "0.0.0.0", cfg.String("server.host"))
	assert.Equal(t, 9090, cfg.Int("server.port"))
	assert.Equal(t, "mysql", cfg.String("database.driver"))
	assert.Equal(t, 100, cfg.Int("database.max_connections"))
}

// TestIntegration_MultipleSources tests merging configurations from multiple sources.
func TestIntegration_MultipleSources(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Base configuration (defaults)
	baseFile := filepath.Join(tmpDir, "base.yaml")
	baseContent := []byte(`
server:
  host: localhost
  port: 8080
  timeout: 30
database:
  pool_size: 10
  timeout: 5
`)
	err := os.WriteFile(baseFile, baseContent, 0o600)
	require.NoError(t, err)

	// Environment-specific override
	envFile := filepath.Join(tmpDir, "production.yaml")
	envContent := []byte(`
server:
  host: 0.0.0.0
  port: 80
database:
  pool_size: 50
`)
	err = os.WriteFile(envFile, envContent, 0o600)
	require.NoError(t, err)

	// Local overrides
	localFile := filepath.Join(tmpDir, "local.yaml")
	localContent := []byte(`
server:
  port: 9090
`)
	err = os.WriteFile(localFile, localContent, 0o600)
	require.NoError(t, err)

	// Load all sources (later sources override earlier ones)
	cfg, err := config.New(
		config.WithFileSource(baseFile, codec.TypeYAML),
		config.WithFileSource(envFile, codec.TypeYAML),
		config.WithFileSource(localFile, codec.TypeYAML),
	)
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// Verify merged values
	assert.Equal(t, "0.0.0.0", cfg.String("server.host")) // from production
	assert.Equal(t, 9090, cfg.Int("server.port"))         // from local (highest priority)
	assert.Equal(t, 30, cfg.Int("server.timeout"))        // from base (not overridden)
	assert.Equal(t, 50, cfg.Int("database.pool_size"))    // from production
	assert.Equal(t, 5, cfg.Int("database.timeout"))       // from base (not overridden)
}

// TestIntegration_BindingWithValidation tests struct binding with validation.
func TestIntegration_BindingWithValidation(t *testing.T) {
	t.Parallel()

	type ServerConfig struct {
		Host string `config:"host"`
		Port int    `config:"port"`
	}

	type DatabaseConfig struct {
		Driver   string `config:"driver"`
		Host     string `config:"host"`
		Port     int    `config:"port"`
		Username string `config:"username"`
		Password string `config:"password"`
	}

	type AppConfig struct {
		Server   ServerConfig   `config:"server"`
		Database DatabaseConfig `config:"database"`
	}

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := []byte(`
server:
  host: localhost
  port: 8080
database:
  driver: postgres
  host: localhost
  port: 5432
  username: testuser
  password: testpass
`)

	err := os.WriteFile(configFile, yamlContent, 0o600)
	require.NoError(t, err)

	var appConfig AppConfig
	cfg, err := config.New(
		config.WithFileSource(configFile, codec.TypeYAML),
		config.WithBinding(&appConfig),
	)
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// Verify bound struct
	assert.Equal(t, "localhost", appConfig.Server.Host)
	assert.Equal(t, 8080, appConfig.Server.Port)
	assert.Equal(t, "postgres", appConfig.Database.Driver)
	assert.Equal(t, "localhost", appConfig.Database.Host)
	assert.Equal(t, 5432, appConfig.Database.Port)
	assert.Equal(t, "testuser", appConfig.Database.Username)
	assert.Equal(t, "testpass", appConfig.Database.Password)
}

// TestIntegration_ReloadConfiguration tests reloading configuration.
func TestIntegration_ReloadConfiguration(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Initial configuration
	initialContent := []byte(`
version: 1
feature_flags:
  new_ui: false
`)

	err := os.WriteFile(configFile, initialContent, 0o600)
	require.NoError(t, err)

	cfg, err := config.New(
		config.WithFileSource(configFile, codec.TypeYAML),
	)
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, cfg.Int("version"))
	assert.False(t, cfg.Bool("feature_flags.new_ui"))

	// Update configuration file
	updatedContent := []byte(`
version: 2
feature_flags:
  new_ui: true
`)

	err = os.WriteFile(configFile, updatedContent, 0o600)
	require.NoError(t, err)

	// Reload configuration
	err = cfg.Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 2, cfg.Int("version"))
	assert.True(t, cfg.Bool("feature_flags.new_ui"))
}

// TestIntegration_FileDumper tests dumping configuration to a file.
func TestIntegration_FileDumper(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "source.yaml")
	dumpFile := filepath.Join(tmpDir, "dump.yaml")

	sourceContent := []byte(`
app:
  name: TestApp
  version: 1.0.0
`)

	err := os.WriteFile(sourceFile, sourceContent, 0o600)
	require.NoError(t, err)

	cfg, err := config.New(
		config.WithFileSource(sourceFile, codec.TypeYAML),
		config.WithFileDumperAs(dumpFile, codec.TypeYAML),
	)
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// Dump configuration
	err = cfg.Dump(context.Background())
	require.NoError(t, err)

	// Verify dumped file exists and contains correct data
	//nolint:gosec // Test file read is safe
	dumpedContent, err := os.ReadFile(dumpFile)
	require.NoError(t, err)
	assert.Contains(t, string(dumpedContent), "TestApp")
	assert.Contains(t, string(dumpedContent), "1.0.0")
}

// TestIntegration_CaseInsensitiveKeys tests case-insensitive key access.
func TestIntegration_CaseInsensitiveKeys(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	yamlContent := []byte(`
Server:
  Host: localhost
  Port: 8080
Database:
  Driver: postgres
`)

	err := os.WriteFile(configFile, yamlContent, 0o600)
	require.NoError(t, err)

	cfg, err := config.New(
		config.WithFileSource(configFile, codec.TypeYAML),
	)
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// All case variations should work
	assert.Equal(t, "localhost", cfg.String("server.host"))
	assert.Equal(t, "localhost", cfg.String("Server.Host"))
	assert.Equal(t, "localhost", cfg.String("SERVER.HOST"))
	assert.Equal(t, 8080, cfg.Int("server.port"))
	assert.Equal(t, 8080, cfg.Int("Server.Port"))
	assert.Equal(t, "postgres", cfg.String("database.driver"))
	assert.Equal(t, "postgres", cfg.String("DATABASE.DRIVER"))
}

// TestIntegration_EnvironmentVariables tests environment variable source.
func TestIntegration_EnvironmentVariables(t *testing.T) {
	// NOTE: Cannot use t.Parallel() with t.Setenv()

	// Set test environment variables
	t.Setenv("TESTAPP_SERVER_HOST", "envhost")
	t.Setenv("TESTAPP_SERVER_PORT", "9090")
	t.Setenv("TESTAPP_DEBUG", "true")

	cfg, err := config.New(
		config.WithOSEnvVarSource("TESTAPP_"),
	)
	require.NoError(t, err)

	err = cfg.Load(context.Background())
	require.NoError(t, err)

	// Environment variables should be accessible with dot notation
	assert.Equal(t, "envhost", cfg.String("server.host"))
	assert.Equal(t, "9090", cfg.String("server.port"))
	assert.Equal(t, "true", cfg.String("debug"))
}
