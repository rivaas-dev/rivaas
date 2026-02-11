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

package router_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rivaas.dev/router"
)

// TestServeAndShutdown covers Serve, defaultServerTimeouts, and Shutdown.
func TestServeAndShutdown(t *testing.T) {
	t.Parallel()
	r := router.MustNew()
	r.GET("/health", func(c *router.Context) {
		//nolint:errcheck // Test handler
		c.String(http.StatusOK, "ok")
	})
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(lis.Addr().String())
	require.NoError(t, err)
	require.NoError(t, lis.Close())

	done := make(chan error, 1)
	go func() {
		done <- r.Serve("127.0.0.1:" + port)
	}()
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://127.0.0.1:" + port + "/health")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, r.Shutdown(ctx))
	assert.Equal(t, http.ErrServerClosed, <-done)
}
