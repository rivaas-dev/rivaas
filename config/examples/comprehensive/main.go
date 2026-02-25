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
	"time"

	"rivaas.dev/config"
)

// WebAppConfig represents a complete web application configuration
// that can be populated from environment variables
type WebAppConfig struct {
	Server     ServerConfig     `config:"server"`
	Database   DatabaseConfig   `config:"database"`
	Redis      RedisConfig      `config:"redis"`
	Auth       AuthConfig       `config:"auth"`
	Logging    LoggingConfig    `config:"logging"`
	Monitoring MonitoringConfig `config:"monitoring"`
	Features   FeaturesConfig   `config:"features"`
}

// ServerConfig represents server configuration settings
type ServerConfig struct {
	Host         string        `config:"host"`
	Port         int           `config:"port"`
	ReadTimeout  time.Duration `config:"read.timeout"`
	WriteTimeout time.Duration `config:"write.timeout"`
	TLS          TLSConfig     `config:"tls"`
}

// TLSConfig represents TLS/SSL configuration settings
type TLSConfig struct {
	Enabled bool       `config:"enabled"`
	Cert    CertConfig `config:"cert"`
	Key     KeyConfig  `config:"key"`
}

// CertConfig represents TLS certificate configuration
type CertConfig struct {
	File string `config:"file"`
}

// KeyConfig represents TLS private key configuration
type KeyConfig struct {
	File string `config:"file"`
}

// DatabaseConfig represents database configuration settings
type DatabaseConfig struct {
	Primary PrimaryConfig `config:"primary"`
	Replica ReplicaConfig `config:"replica"`
	Pool    PoolConfig    `config:"pool"`
}

// PrimaryConfig represents primary database connection settings
type PrimaryConfig struct {
	Host     string `config:"host"`
	Port     int    `config:"port"`
	Database string `config:"database"`
	Username string `config:"username"`
	Password string `config:"password"` //nolint:gosec // G117: example config struct
	SSLMode  string `config:"ssl.mode"`
}

// ReplicaConfig represents replica database connection settings
type ReplicaConfig struct {
	Host     string `config:"host"`
	Port     int    `config:"port"`
	Database string `config:"database"`
	Username string `config:"username"`
	Password string `config:"password"` //nolint:gosec // G117: example config struct
	SSLMode  string `config:"ssl.mode"`
}

// PoolConfig represents database connection pool settings
type PoolConfig struct {
	Max MaxConfig `config:"max"`
}

// MaxConfig represents maximum connection pool limits
type MaxConfig struct {
	Open     int           `config:"open"`
	Idle     int           `config:"idle"`
	Lifetime time.Duration `config:"lifetime"`
}

// RedisConfig represents Redis connection settings
type RedisConfig struct {
	Host     string        `config:"host"`
	Port     int           `config:"port"`
	Password string        `config:"password"` //nolint:gosec // G117: example config struct
	Database int           `config:"database"`
	Timeout  time.Duration `config:"timeout"`
}

// AuthConfig represents authentication configuration settings
type AuthConfig struct {
	JWT     JWTConfig     `config:"jwt"`
	Token   TokenConfig   `config:"token"`
	Refresh RefreshConfig `config:"refresh"`
}

// JWTConfig represents JWT authentication settings
type JWTConfig struct {
	Secret string `config:"secret"` //nolint:gosec // G117: example config struct
}

// TokenConfig represents token configuration settings
type TokenConfig struct {
	Duration time.Duration `config:"duration"`
}

// RefreshConfig represents refresh token configuration settings
type RefreshConfig struct {
	Secret string `config:"secret"` //nolint:gosec // G117: example config struct
}

// LoggingConfig represents logging configuration settings
type LoggingConfig struct {
	Level      string `config:"level"`
	Format     string `config:"format"`
	OutputFile string `config:"output.file"`
}

// MonitoringConfig represents monitoring and metrics configuration
type MonitoringConfig struct {
	Enabled     bool   `config:"enabled"`
	MetricsPort int    `config:"metrics.port"`
	HealthPath  string `config:"health.path"`
}

// FeaturesConfig represents feature flags and settings
type FeaturesConfig struct {
	RateLimit RateLimitConfig `config:"rate.limit"`
	Cache     CacheConfig     `config:"cache"`
	Debug     DebugConfig     `config:"debug"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled bool `config:"enabled"`
}

// CacheConfig represents caching configuration
type CacheConfig struct {
	Enabled bool `config:"enabled"`
}

// DebugConfig represents debug mode settings
type DebugConfig struct {
	Mode bool `config:"mode"`
}

// PrintConfig displays the configuration in a readable format
func (c *WebAppConfig) PrintConfig() {
	fmt.Println("=== Web Application Configuration (YAML + Environment Variables) ===")
	fmt.Printf("Server: %s:%d\n", c.Server.Host, c.Server.Port)
	fmt.Printf("  Read Timeout: %v\n", c.Server.ReadTimeout)
	fmt.Printf("  Write Timeout: %v\n", c.Server.WriteTimeout)
	fmt.Printf("  TLS Enabled: %t\n", c.Server.TLS.Enabled)
	if c.Server.TLS.Enabled {
		fmt.Printf("  TLS Cert: %s\n", c.Server.TLS.Cert.File)
		fmt.Printf("  TLS Key: %s\n", c.Server.TLS.Key.File)
	}

	fmt.Printf("\nDatabase Primary: %s:%d/%s\n",
		c.Database.Primary.Host, c.Database.Primary.Port, c.Database.Primary.Database)
	fmt.Printf("Database Replica: %s:%d/%s\n",
		c.Database.Replica.Host, c.Database.Replica.Port, c.Database.Replica.Database)
	fmt.Printf("Database Pool: MaxOpen=%d, MaxIdle=%d, MaxLifetime=%v\n",
		c.Database.Pool.Max.Open, c.Database.Pool.Max.Idle, c.Database.Pool.Max.Lifetime)

	fmt.Printf("\nRedis: %s:%d (DB: %d)\n", c.Redis.Host, c.Redis.Port, c.Redis.Database)
	fmt.Printf("Redis Timeout: %v\n", c.Redis.Timeout)

	fmt.Printf("\nAuth Token Duration: %v\n", c.Auth.Token.Duration)
	fmt.Printf("Logging Level: %s, Format: %s\n", c.Logging.Level, c.Logging.Format)
	if c.Logging.OutputFile != "" {
		fmt.Printf("Logging Output: %s\n", c.Logging.OutputFile)
	}

	fmt.Printf("\nMonitoring Enabled: %t\n", c.Monitoring.Enabled)
	if c.Monitoring.Enabled {
		fmt.Printf("Metrics Port: %d\n", c.Monitoring.MetricsPort)
		fmt.Printf("Health Path: %s\n", c.Monitoring.HealthPath)
	}

	fmt.Printf("\nFeatures:\n")
	fmt.Printf("  Rate Limit: %t\n", c.Features.RateLimit.Enabled)
	fmt.Printf("  Cache: %t\n", c.Features.Cache.Enabled)
	fmt.Printf("  Debug Mode: %t\n", c.Features.Debug.Mode)
	fmt.Println("=====================================")
}

func main() {
	var wc WebAppConfig

	// Create configuration with multiple sources
	cfg := config.MustNew(
		// First, load from YAML file (default values)
		config.WithFile("config.yaml"),
		// Then, override with environment variables (higher precedence)
		config.WithEnv("WEBAPP_"),
		// Bind to our struct
		config.WithBinding(&wc),
	)

	// Load configuration
	if err := cfg.Load(context.Background()); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Print the loaded configuration
	wc.PrintConfig()

	// Demonstrate accessing configuration values directly
	fmt.Println("\n=== Direct Configuration Access ===")
	serverHost := cfg.String("server.host")
	serverPort := cfg.Int("server.port")
	databaseHost := cfg.String("database.primary.host")

	fmt.Printf("Server: %s:%d\n", serverHost, serverPort)
	fmt.Printf("Database: %s\n", databaseHost)

	// Check if TLS is enabled
	if tlsEnabled := cfg.Bool("server.tls.enabled"); tlsEnabled {
		fmt.Println("TLS is enabled")
	} else {
		fmt.Println("TLS is disabled")
	}

	// Demonstrate configuration precedence
	fmt.Println("\n=== Configuration Precedence Demo ===")
	fmt.Println("Values are loaded in this order:")
	fmt.Println("1. YAML file (config.yaml) - default values")
	fmt.Println("2. Environment variables (WEBAPP_*) - override defaults")
	fmt.Println("")
	fmt.Println("Example: If YAML has server.port=3000 and env has WEBAPP_SERVER_PORT=8080")
	fmt.Println("The final value will be 8080 (environment variable wins)")
}
