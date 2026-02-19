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

//go:build integration

package accesslog_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestAccesslogIntegration is the entry point for the accesslog integration test suite.
//
// Integration tests verify that accesslog works correctly with other middleware
// in realistic scenarios.
//
//nolint:paralleltest // Integration test suite
func TestAccesslogIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AccessLog Integration Suite")
}
