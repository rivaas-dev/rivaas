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

package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"

	"rivaas.dev/openapi"
	"rivaas.dev/openapi/diag"
)

// openapiState manages OpenAPI specification state for the app.
// This replaces the openapi.Manager with app-local state management.
type openapiState struct {
	api *openapi.API
	ops []openapi.Operation

	// Cache
	specCache []byte
	specETag  string
	warnings  diag.Warnings

	mu sync.RWMutex
}

// newOpenapiState creates a new OpenAPI state manager.
func newOpenapiState(api *openapi.API) *openapiState {
	return &openapiState{
		api: api,
		ops: make([]openapi.Operation, 0),
	}
}

// AddOperation adds an operation to the OpenAPI spec.
// This invalidates the cached spec.
func (s *openapiState) AddOperation(op openapi.Operation) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ops = append(s.ops, op)

	// Invalidate cache
	s.specCache = nil
	s.specETag = ""
	s.warnings = nil
}

// GenerateSpec generates the OpenAPI specification.
// Results are cached until a new operation is added.
func (s *openapiState) GenerateSpec(ctx context.Context) ([]byte, string, error) {
	// Fast path: check cache with read lock
	s.mu.RLock()
	if s.specCache != nil {
		cache, etag := s.specCache, s.specETag
		s.mu.RUnlock()
		return cache, etag, nil
	}
	s.mu.RUnlock()

	// Slow path: generate with write lock
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.specCache != nil {
		return s.specCache, s.specETag, nil
	}

	// Generate spec using API method
	result, err := s.api.Generate(ctx, s.ops...)
	if err != nil {
		return nil, "", err
	}

	// Cache the result
	s.specCache = result.JSON
	s.specETag = fmt.Sprintf(`"%x"`, sha256.Sum256(result.JSON))
	s.warnings = result.Warnings

	return s.specCache, s.specETag, nil
}

// Warnings returns warnings from the last successful spec generation.
func (s *openapiState) Warnings() diag.Warnings {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.warnings == nil {
		return nil
	}

	// Return a copy
	result := make(diag.Warnings, len(s.warnings))
	copy(result, s.warnings)
	return result
}

// API returns the OpenAPI configuration.
// Safe without lock: api is immutable after construction.
func (s *openapiState) API() *openapi.API {
	return s.api
}

// SpecPath returns the configured spec path (e.g., "/openapi.json").
// Safe without lock: api is immutable after construction.
func (s *openapiState) SpecPath() string {
	return s.api.SpecPath
}

// UIPath returns the configured UI path (e.g., "/docs").
// Safe without lock: api is immutable after construction.
func (s *openapiState) UIPath() string {
	return s.api.UIPath
}

// ServeUI returns whether Swagger UI should be served.
// Safe without lock: api is immutable after construction.
func (s *openapiState) ServeUI() bool {
	return s.api.ServeUI
}

// UIConfig returns the UI configuration for rendering Swagger UI.
// Safe without lock: api is immutable after construction.
func (s *openapiState) UIConfig() openapi.UIConfig {
	return s.api.UI()
}
