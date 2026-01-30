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
	"log"

	"rivaas.dev/config"
)

// SimpleConfig represents a simple configuration without validation
type SimpleConfig struct {
	Server   ServerConfig   `config:"server"`
	Database DatabaseConfig `config:"database"`
	Auth     AuthConfig     `config:"auth"`
	Features FeaturesConfig `config:"features"`
}

// ServerConfig represents server configuration settings
type ServerConfig struct {
	Host string `config:"host"`
	Port int    `config:"port"`
}

// DatabaseConfig represents database configuration settings
type DatabaseConfig struct {
	Primary PrimaryConfig `config:"primary"`
}

// PrimaryConfig represents primary database connection settings
type PrimaryConfig struct {
	Host     string `config:"host"`
	Port     int    `config:"port"`
	Database string `config:"database"`
}

// AuthConfig represents authentication configuration settings
type AuthConfig struct {
	JWT JWTConfig `config:"jwt"`
}

// JWTConfig represents JWT authentication settings
type JWTConfig struct {
	Secret string `config:"secret"`
}

// FeaturesConfig represents feature flags and settings
type FeaturesConfig struct {
	Debug DebugConfig `config:"debug"`
}

// DebugConfig represents debug mode settings
type DebugConfig struct {
	Mode bool `config:"mode"`
}

// PrintConfig displays the configuration in a readable format
func (c *SimpleConfig) PrintConfig() {
	fmt.Println("=== Simple Configuration ===")
	fmt.Printf("Server: %s:%d\n", c.Server.Host, c.Server.Port)
	fmt.Printf("Database: %s:%d/%s\n", c.Database.Primary.Host, c.Database.Primary.Port, c.Database.Primary.Database)
	fmt.Printf("Auth JWT Secret: %s\n", c.Auth.JWT.Secret)
	fmt.Printf("Debug Mode: %t\n", c.Features.Debug.Mode)
	fmt.Println("============================")
}

func main() {
	var sc SimpleConfig

	// Create configuration with environment variable source
	cfg := config.MustNew(
		config.WithEnv("WEBAPP_"),
		config.WithBinding(&sc),
	)

	// Load configuration
	if err := cfg.Load(context.Background()); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Print the loaded configuration
	sc.PrintConfig()

	// Demonstrate accessing configuration values directly
	fmt.Println("\n=== Direct Configuration Access ===")
	serverHost := cfg.String("server.host")
	serverPort := cfg.Int("server.port")
	databaseHost := cfg.String("database.primary.host")

	fmt.Printf("Server: %s:%d\n", serverHost, serverPort)
	fmt.Printf("Database: %s\n", databaseHost)

	// Check if debug mode is enabled
	if debugMode := cfg.Bool("features.debug.mode"); debugMode {
		fmt.Println("Debug mode is enabled")
	} else {
		fmt.Println("Debug mode is disabled")
	}
}
