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

package middleware_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"

	"rivaas.dev/router"
	"rivaas.dev/router/middleware/accesslog"
	"rivaas.dev/router/middleware/recovery"
	"rivaas.dev/router/middleware/requestid"
	"rivaas.dev/router/middleware/security"
)

// Example_basicChain demonstrates building a router with common middlewares:
// recovery, requestid, accesslog, and security.
func Example_basicChain() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	r := router.MustNew()
	r.Use(recovery.New())
	r.Use(requestid.New())
	r.Use(accesslog.New(accesslog.WithLogger(logger)))
	r.Use(security.New())

	r.GET("/health", func(c *router.Context) {
		//nolint:errcheck // Example
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	fmt.Println(w.Body.String())
	fmt.Println("status:", w.Code)
	// Output:
	// OK
	// status: 200
}
