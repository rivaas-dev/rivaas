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

// Package config provides configuration management for Go applications.
//
// The config package loads configuration data from multiple sources including
// files, environment variables, and remote systems like Consul. Configuration
// sources are merged in order, with later sources overriding earlier ones.
// All configuration keys are case-insensitive.
//
// # Key Features
//
//   - Multiple configuration sources (files, environment variables, Consul)
//   - Automatic format detection and decoding (JSON, YAML, TOML)
//   - Struct binding with automatic type conversion
//   - Validation using JSON Schema or custom validators
//   - Case-insensitive key access with dot notation
//   - Thread-safe configuration loading and access
//   - Configuration dumping to files or custom destinations
//
// # Quick Start
//
// Create a configuration instance with sources:
//
//	cfg := config.MustNew(
//	    config.WithFile("config.yaml"),
//	    config.WithEnv("APP_"),
//	)
//
// Load the configuration:
//
//	if err := cfg.Load(context.Background()); err != nil {
//	    log.Fatal(err)
//	}
//
// Access configuration values:
//
//	port := cfg.Int("server.port")
//	host := cfg.StringOr("server.host", "localhost")
//	debug := cfg.Bool("debug")
//
// # Configuration Sources
//
// The package supports multiple configuration sources that can be combined:
//
// Files with automatic format detection:
//
//	config.WithFile("config.yaml")     // Detects YAML
//	config.WithFile("config.json")     // Detects JSON
//	config.WithFile("config.toml")     // Detects TOML
//
// Files with explicit format:
//
//	config.WithFileAs("config", codec.TypeYAML)
//
// Environment variables with prefix:
//
//	config.WithEnv("APP_")  // Loads APP_SERVER_PORT as server.port
//
// Consul key-value store:
//
//	config.WithConsul("production/service.yaml")
//
// Raw content:
//
//	yamlData := []byte("port: 8080")
//	config.WithContent(yamlData, codec.TypeYAML)
//
// # Struct Binding
//
// Bind configuration to a struct for type-safe access:
//
//	type AppConfig struct {
//	    Port    int           `config:"port"`
//	    Host    string        `config:"host"`
//	    Timeout time.Duration `config:"timeout"`
//	    Debug   bool          `config:"debug" default:"false"`
//	}
//
//	var appConfig AppConfig
//	cfg := config.MustNew(
//	    config.WithFile("config.yaml"),
//	    config.WithBinding(&appConfig),
//	)
//
//	if err := cfg.Load(context.Background()); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access typed fields directly
//	fmt.Printf("Server: %s:%d\n", appConfig.Host, appConfig.Port)
//
// # Validation
//
// Validate configuration using struct methods:
//
//	type Config struct {
//	    Port int `config:"port"`
//	}
//
//	func (c *Config) Validate() error {
//	    if c.Port < 1 || c.Port > 65535 {
//	        return fmt.Errorf("port must be between 1 and 65535")
//	    }
//	    return nil
//	}
//
// Validate using JSON Schema:
//
//	schema := []byte(`{
//	    "type": "object",
//	    "properties": {
//	        "port": {"type": "integer", "minimum": 1, "maximum": 65535}
//	    },
//	    "required": ["port"]
//	}`)
//
//	cfg := config.MustNew(
//	    config.WithFile("config.yaml"),
//	    config.WithJSONSchema(schema),
//	)
//
// Validate using custom functions:
//
//	cfg := config.MustNew(
//	    config.WithFile("config.yaml"),
//	    config.WithValidator(func(values map[string]any) error {
//	        if port, ok := values["port"].(int); ok && port < 1 {
//	            return fmt.Errorf("invalid port: %d", port)
//	        }
//	        return nil
//	    }),
//	)
//
// # Accessing Configuration Values
//
// Access values using type-specific methods:
//
//	// Basic types
//	port := cfg.Int("server.port")
//	host := cfg.String("server.host")
//	debug := cfg.Bool("debug")
//	rate := cfg.Float64("rate")
//
//	// With default values
//	host := cfg.StringOr("server.host", "localhost")
//	port := cfg.IntOr("server.port", 8080)
//
//	// Collections
//	tags := cfg.StringSlice("tags")
//	ports := cfg.IntSlice("ports")
//	metadata := cfg.StringMap("metadata")
//
//	// Time-related
//	timeout := cfg.Duration("timeout")
//	startTime := cfg.Time("start_time")
//
// Using generic functions with error handling:
//
//	port, err := config.GetE[int](cfg, "server.port")
//	if err != nil {
//	    log.Fatalf("port configuration required: %v", err)
//	}
//
// # Configuration Dumping
//
// Save the current configuration to a file:
//
//	cfg := config.MustNew(
//	    config.WithFile("config.yaml"),
//	    config.WithFileDumper("output.yaml"),
//	)
//
//	cfg.Load(context.Background())
//	cfg.Dump(context.Background())  // Writes to output.yaml
//
// # Thread Safety
//
// Config is safe for concurrent use by multiple goroutines.
// Configuration loading and reading are protected by internal locks.
// Multiple goroutines can safely call Load() and access configuration
// values simultaneously.
//
// # Error Handling
//
// The package provides detailed error information through [ConfigError]:
//
//	if err := cfg.Load(ctx); err != nil {
//	    var configErr *config.ConfigError
//	    if errors.As(err, &configErr) {
//	        fmt.Printf("Error in %s during %s: %v\n",
//	            configErr.Source, configErr.Operation, configErr.Err)
//	    }
//	}
//
// # Examples
//
// See the examples directory for complete working examples demonstrating
// various configuration patterns and use cases including:
//
//   - Basic configuration loading from files
//   - Environment variable overrides
//   - Struct binding with validation
//   - JSON Schema validation
//   - Custom validation functions
//   - Configuration dumping
//   - Consul integration
//
// For more details, see the package documentation at https://pkg.go.dev/rivaas.dev/config
package config
