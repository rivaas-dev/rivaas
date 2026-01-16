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

// Package middleware_test provides integration tests for the router middleware stack.
//
// # Integration Testing Strategy
//
// This package uses BDD-style integration tests with Ginkgo/Gomega to verify that
// middleware components work correctly together in realistic scenarios. Integration
// tests differ from unit tests in that they:
//
//   - Test multiple middleware components together
//   - Verify end-to-end behavior with real HTTP requests
//   - Test middleware interaction and ordering
//   - Validate real-world use cases and scenarios
//
// # Test Structure
//
//   - middleware_integration_suite_test.go: Test suite setup (this file)
//   - integration_test.go: Integration test cases
//
// # Running Integration Tests
//
// Run all tests (including integration):
//
//	go test ./middleware/...
//
// Skip integration tests (faster, for TDD):
//
//	go test -short ./middleware/...
//
// Run only integration tests:
//
//	go test -run TestMiddlewareIntegration ./middleware/...
//
// Run with verbose output:
//
//	go test -v ./middleware/...
//
// # Test Categories
//
// Integration tests are organized into these categories:
//
//  1. Basic Stack: RequestID + AccessLog + Recovery
//  2. Security Stack: CORS + Security Headers + BasicAuth
//  3. Performance Stack: Compression + Caching
//  4. Full Stack: All middleware combined
//
// # Related Documentation
//
//   - Unit tests: See *_test.go files in individual middleware packages
//   - Middleware docs: See middleware/README.md
//   - Testing standards: See docs/TESTING_STANDARDS.md

//go:build integration

package middleware_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestMiddlewareIntegration is the entry point for the middleware integration test suite.
//
// Integration tests are skipped when running with -short flag to allow fast TDD cycles
// with unit tests only.
//
//nolint:paralleltest // Integration test suite
func TestMiddlewareIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Middleware Integration Suite")
}
