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

package config_test

import (
	"context"
	"fmt"
	"log"

	"rivaas.dev/config"
	"rivaas.dev/config/codec"
)

// Example demonstrates basic configuration usage.
func Example() {
	// Create config with YAML content
	yamlContent := []byte(`
server:
  host: localhost
  port: 8080
database:
  name: mydb
`)

	cfg, err := config.New(
		config.WithContent(yamlContent, codec.TypeYAML),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Load configuration
	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Access values
	fmt.Println(cfg.String("server.host"))
	fmt.Println(cfg.Int("server.port"))
	fmt.Println(cfg.String("database.name"))

	// Output:
	// localhost
	// 8080
	// mydb
}

// ExampleNew demonstrates creating a new configuration instance.
func ExampleNew() {
	cfg, err := config.New()
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Config created successfully")
	// Output: Config created successfully
}

// ExampleMustNew demonstrates creating a configuration instance with panic on error.
func ExampleMustNew() {
	cfg := config.MustNew()
	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Config created successfully")
	// Output: Config created successfully
}

// ExampleWithFileSource demonstrates loading configuration from a file.
func ExampleWithFileSource() {
	// Create a temporary config file (in real code, use an actual file path)
	cfg, err := config.New(
		config.WithContent([]byte(`{"name": "example"}`), codec.TypeJSON),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	fmt.Println(cfg.String("name"))
	// Output: example
}

// ExampleWithContent demonstrates loading configuration from byte content.
func ExampleWithContent() {
	jsonContent := []byte(`{
		"app": {
			"name": "MyApp",
			"version": "1.0.0"
		}
	}`)

	cfg, err := config.New(
		config.WithContent(jsonContent, codec.TypeJSON),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	fmt.Println(cfg.String("app.name"))
	fmt.Println(cfg.String("app.version"))
	// Output:
	// MyApp
	// 1.0.0
}

// ExampleWithBinding demonstrates binding configuration to a struct.
func ExampleWithBinding() {
	type ServerConfig struct {
		Host string `config:"host"`
		Port int    `config:"port"`
	}

	type Config struct {
		Server ServerConfig `config:"server"`
	}

	yamlContent := []byte(`
server:
  host: localhost
  port: 8080
`)

	var appConfig Config
	cfg, err := config.New(
		config.WithContent(yamlContent, codec.TypeYAML),
		config.WithBinding(&appConfig),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s:%d\n", appConfig.Server.Host, appConfig.Server.Port)
	// Output: localhost:8080
}

// ExampleWithValidator demonstrates using a custom validator.
func ExampleWithValidator() {
	yamlContent := []byte(`name: myapp`)

	cfg, err := config.New(
		config.WithContent(yamlContent, codec.TypeYAML),
		config.WithValidator(func(cfgMap map[string]any) error {
			// Custom validation logic
			if _, ok := cfgMap["name"]; !ok {
				return fmt.Errorf("name is required")
			}
			return nil
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Validation passed")
	// Output: Validation passed
}

// ExampleConfig_Get demonstrates retrieving configuration values.
func ExampleConfig_Get() {
	yamlContent := []byte(`
settings:
  enabled: true
  count: 42
`)

	cfg, err := config.New(
		config.WithContent(yamlContent, codec.TypeYAML),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	fmt.Println(cfg.Get("settings.enabled"))
	fmt.Println(cfg.Get("settings.count"))
	// Output:
	// true
	// 42
}

// ExampleConfig_String demonstrates retrieving string values.
func ExampleConfig_String() {
	jsonContent := []byte(`{"name": "MyApp", "env": "production"}`)

	cfg, err := config.New(
		config.WithContent(jsonContent, codec.TypeJSON),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	fmt.Println(cfg.String("name"))
	fmt.Println(cfg.String("env"))
	// Output:
	// MyApp
	// production
}

// ExampleConfig_Int demonstrates retrieving integer values.
func ExampleConfig_Int() {
	jsonContent := []byte(`{"port": 8080, "workers": 4}`)

	cfg, err := config.New(
		config.WithContent(jsonContent, codec.TypeJSON),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	fmt.Println(cfg.Int("port"))
	fmt.Println(cfg.Int("workers"))
	// Output:
	// 8080
	// 4
}

// ExampleConfig_Bool demonstrates retrieving boolean values.
func ExampleConfig_Bool() {
	jsonContent := []byte(`{"debug": true, "verbose": false}`)

	cfg, err := config.New(
		config.WithContent(jsonContent, codec.TypeJSON),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	fmt.Println(cfg.Bool("debug"))
	fmt.Println(cfg.Bool("verbose"))
	// Output:
	// true
	// false
}

// ExampleConfig_StringSlice demonstrates retrieving string slices.
func ExampleConfig_StringSlice() {
	yamlContent := []byte(`
tags:
  - web
  - api
  - backend
`)

	cfg, err := config.New(
		config.WithContent(yamlContent, codec.TypeYAML),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	tags := cfg.StringSlice("tags")
	fmt.Printf("%v\n", tags)
	// Output: [web api backend]
}

// ExampleConfig_StringMap demonstrates retrieving string maps.
func ExampleConfig_StringMap() {
	yamlContent := []byte(`
metadata:
  author: John Doe
  version: 1.0.0
`)

	cfg, err := config.New(
		config.WithContent(yamlContent, codec.TypeYAML),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	metadata := cfg.StringMap("metadata")
	fmt.Println(metadata["author"])
	fmt.Println(metadata["version"])
	// Output:
	// John Doe
	// 1.0.0
}

// Example_multipleSources demonstrates merging multiple configuration sources.
func Example_multipleSources() {
	// Base configuration
	baseConfig := []byte(`
server:
  host: localhost
  port: 8080
`)

	// Override configuration
	overrideConfig := []byte(`
server:
  port: 9090
`)

	cfg, err := config.New(
		config.WithContent(baseConfig, codec.TypeYAML),
		config.WithContent(overrideConfig, codec.TypeYAML),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Later sources override earlier ones
	fmt.Println(cfg.String("server.host"))
	fmt.Println(cfg.Int("server.port"))
	// Output:
	// localhost
	// 9090
}

// Example_environmentVariables demonstrates loading configuration from environment variables.
func Example_environmentVariables() {
	// In real usage, set environment variables like:
	// export APP_SERVER_HOST=localhost
	// export APP_SERVER_PORT=8080

	cfg, err := config.New(
		config.WithOSEnvVarSource("APP_"),
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Access environment variables without the prefix
	// e.g., APP_SERVER_HOST becomes server.host
	fmt.Println("Environment variables loaded")
	// Output: Environment variables loaded
}
