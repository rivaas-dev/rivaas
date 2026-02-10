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

//go:build !integration

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"rivaas.dev/openapi"
)

func TestWithBefore_appendsHandlers(t *testing.T) {
	t.Parallel()

	h1 := func(c *Context) {}
	h2 := func(c *Context) {}
	cfg := &routeConfig{}
	WithBefore(h1, h2)(cfg)
	assert.Len(t, cfg.before, 2)
	assert.NotNil(t, cfg.before[0])
	assert.NotNil(t, cfg.before[1])
}

func TestWithAfter_appendsHandlers(t *testing.T) {
	t.Parallel()

	h1 := func(c *Context) {}
	cfg := &routeConfig{}
	WithAfter(h1)(cfg)
	assert.Len(t, cfg.after, 1)
	assert.NotNil(t, cfg.after[0])
}

func TestWithDoc_appendsDocOpts(t *testing.T) {
	t.Parallel()

	cfg := &routeConfig{}
	WithDoc(openapi.WithSummary("Get user"), openapi.WithDescription("Retrieves a user"))(cfg)
	assert.Len(t, cfg.docOpts, 2)
	assert.False(t, cfg.skipDoc)
}

func TestWithoutDoc_setsSkipDocTrue(t *testing.T) {
	t.Parallel()

	cfg := &routeConfig{}
	WithoutDoc()(cfg)
	assert.True(t, cfg.skipDoc)
}

func TestRouteOptions_appliesMultipleOptions(t *testing.T) {
	t.Parallel()

	h := func(c *Context) {}
	cfg := &routeConfig{}
	composite := RouteOptions(
		WithBefore(h),
		WithDoc(openapi.WithSummary("test")),
	)
	composite(cfg)
	assert.Len(t, cfg.before, 1)
	assert.Len(t, cfg.docOpts, 1)
	assert.NotNil(t, cfg.before[0])
}

func TestRouteOptions_accumulatesMultipleCalls(t *testing.T) {
	t.Parallel()

	h1 := func(c *Context) {}
	h2 := func(c *Context) {}
	cfg := &routeConfig{}
	WithBefore(h1)(cfg)
	WithBefore(h2)(cfg)
	WithDoc(openapi.WithSummary("a"))(cfg)
	WithDoc(openapi.WithSummary("b"))(cfg)
	assert.Len(t, cfg.before, 2)
	assert.Len(t, cfg.docOpts, 2)
}
