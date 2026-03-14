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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

func TestContextPool_Put_resetsContext(t *testing.T) {
	t.Parallel()

	a, err := New()
	require.NoError(t, err)

	rc := &router.Context{
		Request:  httptest.NewRequest(http.MethodGet, "/", nil),
		Response: httptest.NewRecorder(),
	}

	c := a.contextPool.Get()
	require.NotNil(t, c)

	// Set all fields to non-nil so we can verify Put clears them
	c.Context = rc
	c.app = a
	c.bindingMeta = &bindingMetadata{}

	a.contextPool.Put(c)

	// Get from pool again; we may receive the same instance
	reused := a.contextPool.Get()
	require.NotNil(t, reused)

	// Put clears Context, app, and bindingMeta so pooled instances are reset
	assert.Nil(t, reused.Context, "Context should be nil after Put")
	assert.Nil(t, reused.app, "app should be nil after Put")
	assert.Nil(t, reused.bindingMeta, "bindingMeta should be nil after Put")

	a.contextPool.Put(reused)
}
