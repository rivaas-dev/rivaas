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

// WebAppConfig represents a web application configuration without validation
type WebAppConfig struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	Auth       AuthConfig
	Logging    LoggingConfig
	Monitoring MonitoringConfig
	Features   FeaturesConfig
}

// ServerConfig represents server configuration settings
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	TLS          TLSConfig
}

// TLSConfig represents TLS/SSL configuration settings
type TLSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
}

// DatabaseConfig represents database configuration settings
type DatabaseConfig struct {
	Primary PrimaryConfig
	Replica ReplicaConfig
	Pool    PoolConfig
}

// PrimaryConfig represents primary database connection settings
type PrimaryConfig struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string //nolint:gosec // G117: example config struct
	SSLMode  string
}

// ReplicaConfig represents replica database connection settings
type ReplicaConfig struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string //nolint:gosec // G117: example config struct
	SSLMode  string
}

// PoolConfig represents database connection pool settings
type PoolConfig struct {
	Max MaxConfig
}

// MaxConfig represents maximum connection pool limits
type MaxConfig struct {
	Open     int
	Idle     int
	Lifetime time.Duration
}

// RedisConfig represents Redis connection settings
type RedisConfig struct {
	Host     string
	Port     int
	Password string //nolint:gosec // G117: example config struct
	Database int
	Timeout  time.Duration
}

// AuthConfig represents authentication configuration settings
type AuthConfig struct {
	JWT     JWTConfig
	Token   TokenConfig
	Refresh RefreshConfig
}

// JWTConfig represents JWT authentication settings
type JWTConfig struct {
	Secret string //nolint:gosec // G117: example config struct
}

// TokenConfig represents token configuration settings
type TokenConfig struct {
	Duration time.Duration
}

// RefreshConfig represents refresh token configuration settings
type RefreshConfig struct {
	Secret string //nolint:gosec // G117: example config struct
}

// LoggingConfig represents logging configuration settings
type LoggingConfig struct {
	Level      string
	Format     string
	OutputFile string
}

// MonitoringConfig represents monitoring and metrics configuration
type MonitoringConfig struct {
	Enabled     bool
	MetricsPort int
	HealthPath  string
}

// FeaturesConfig represents feature flags and settings
type FeaturesConfig struct {
	RateLimit RateLimitConfig
	Cache     CacheConfig
	Debug     DebugConfig
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled bool
}

// CacheConfig represents caching configuration
type CacheConfig struct {
	Enabled bool
}

// DebugConfig represents debug mode settings
type DebugConfig struct {
	Mode bool
}

// PrintConfig displays the configuration in a readable format
func (c *WebAppConfig) PrintConfig() {
	fmt.Println("=== Web Application Configuration (YAML + Environment Variables) ===")
	fmt.Printf("Server: %s:%d\n", c.Server.Host, c.Server.Port)
	fmt.Printf("  Read Timeout: %v\n", c.Server.ReadTimeout)
	fmt.Printf("  Write Timeout: %v\n", c.Server.WriteTimeout)
	fmt.Printf("  TLS Enabled: %t\n", c.Server.TLS.Enabled)
	if c.Server.TLS.Enabled {
		fmt.Printf("  TLS Cert: %s\n", c.Server.TLS.CertFile)
		fmt.Printf("  TLS Key: %s\n", c.Server.TLS.KeyFile)
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
	cfg, err := config.New(
		// First, load from YAML file (default values)
		config.WithFile("config.yaml"),
		// Then, override with environment variables (higher precedence)
		config.WithEnv("WEBAPP_"),
		// Bind to our struct
		config.WithBinding(&wc),
	)
	if err != nil {
		log.Fatalf("Failed to create configuration: %v", err)
	}

	// Load configuration
	if err = cfg.Load(context.Background()); err != nil {
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
	fmt.Println("")
	fmt.Println("Note: Environment variables create lowercase keys. For best compatibility:")
	fmt.Println("- Use simple keys: server.port -> WEBAPP_SERVER_PORT")
	fmt.Println("- For nested keys: database.primary.host -> WEBAPP_DATABASE_PRIMARY_HOST")
	fmt.Println("- Avoid camelCase in YAML if you need env var overrides for those keys")
}
