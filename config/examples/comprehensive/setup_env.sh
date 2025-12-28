#!/bin/bash

# Environment Variable Example Setup Script
# This script sets up all the environment variables needed to run the example

echo "Setting up environment variables for Config package Environment Variable Example..."

# Server Configuration
export WEBAPP_SERVER_HOST=0.0.0.0
export WEBAPP_SERVER_PORT=8080
export WEBAPP_SERVER_READ_TIMEOUT=30s
export WEBAPP_SERVER_WRITE_TIMEOUT=30s
export WEBAPP_SERVER_TLS_ENABLED=false

# Database Configuration
export WEBAPP_DATABASE_PRIMARY_HOST=localhost
export WEBAPP_DATABASE_PRIMARY_PORT=5432
export WEBAPP_DATABASE_PRIMARY_DATABASE=myapp
export WEBAPP_DATABASE_PRIMARY_USERNAME=postgres
export WEBAPP_DATABASE_PRIMARY_PASSWORD=secret123
export WEBAPP_DATABASE_PRIMARY_SSL_MODE=disable

export WEBAPP_DATABASE_REPLICA_HOST=replica.example.com
export WEBAPP_DATABASE_REPLICA_PORT=5432
export WEBAPP_DATABASE_REPLICA_DATABASE=myapp
export WEBAPP_DATABASE_REPLICA_USERNAME=readonly
export WEBAPP_DATABASE_REPLICA_PASSWORD=readonly123
export WEBAPP_DATABASE_REPLICA_SSL_MODE=require

export WEBAPP_DATABASE_POOL_MAX_OPEN=25
export WEBAPP_DATABASE_POOL_MAX_IDLE=5
export WEBAPP_DATABASE_POOL_MAX_LIFETIME=5m

# Redis Configuration
export WEBAPP_REDIS_HOST=localhost
export WEBAPP_REDIS_PORT=6379
export WEBAPP_REDIS_PASSWORD=
export WEBAPP_REDIS_DATABASE=0
export WEBAPP_REDIS_TIMEOUT=5s

# Authentication
export WEBAPP_AUTH_JWT_SECRET=your-super-secret-jwt-key-here
export WEBAPP_AUTH_TOKEN_DURATION=24h
export WEBAPP_AUTH_REFRESH_SECRET=your-refresh-secret-key

# Logging
export WEBAPP_LOGGING_LEVEL=info
export WEBAPP_LOGGING_FORMAT=json
export WEBAPP_LOGGING_OUTPUT_FILE=/var/log/myapp.log

# Monitoring
export WEBAPP_MONITORING_ENABLED=true
export WEBAPP_MONITORING_METRICS_PORT=9090
export WEBAPP_MONITORING_HEALTH_PATH=/health

# Features
export WEBAPP_FEATURES_RATE_LIMIT_ENABLED=true
export WEBAPP_FEATURES_CACHE_ENABLED=true
export WEBAPP_FEATURES_DEBUG_MODE=false

echo "Environment variables set successfully!"
echo ""
echo "You can now run the example with:"
echo "go run main.go"
echo ""
echo "Or source this script in your current shell:"
echo "source setup_env.sh"
echo ""
echo "To see all set environment variables:"
echo "env | grep WEBAPP_" 