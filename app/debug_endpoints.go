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
	"fmt"
	"net/http/pprof"

	"rivaas.dev/router"
	"rivaas.dev/router/route"
)

// registerDebugEndpoints registers debug endpoints based on the provided settings.
// app.New() calls this internally when debug endpoints are configured.
func (a *App) registerDebugEndpoints(s *debugSettings) error {
	prefix := s.prefix
	if prefix == "" {
		prefix = "/_internal/debug"
	}

	// Register MCP debug endpoints if configured
	if s.mcpDebug != nil && s.mcpDebug.enabled {
		if err := a.registerMCPDebugEndpoints(s); err != nil {
			return fmt.Errorf("failed to register MCP debug endpoints: %w", err)
		}
	}

	if !s.pprofEnabled {
		return nil
	}

	base := prefix + "/pprof"

	// Check for route collisions
	pprofPaths := []string{
		base + "/",
		base + "/cmdline",
		base + "/profile",
		base + "/symbol",
		base + "/trace",
		base + "/allocs",
		base + "/block",
		base + "/goroutine",
		base + "/heap",
		base + "/mutex",
		base + "/threadcreate",
	}

	for _, path := range pprofPaths {
		if a.router.RouteExists("GET", path) {
			return fmt.Errorf("route already registered: GET %s", path)
		}
	}

	// Check POST /symbol
	if a.router.RouteExists("POST", base+"/symbol") {
		return fmt.Errorf("route already registered: POST %s/symbol", base)
	}

	// Register pprof endpoints
	registerPprof(a.Router(), base)

	for _, path := range pprofPaths {
		a.router.UpdateRouteInfo("GET", path, "", func(info *route.Info) {
			info.HandlerName = "[builtin] pprof"
		})
	}
	a.router.UpdateRouteInfo("POST", base+"/symbol", "", func(info *route.Info) {
		info.HandlerName = "[builtin] pprof"
	})

	return nil
}

// registerPprof registers all pprof endpoints under the given base path.
func registerPprof(r *router.Router, base string) {
	// Main index
	r.GET(base+"/", func(c *router.Context) {
		pprof.Index(c.Response, c.Request)
	})

	// Common endpoints
	r.GET(base+"/cmdline", func(c *router.Context) {
		pprof.Cmdline(c.Response, c.Request)
	})

	r.GET(base+"/profile", func(c *router.Context) {
		pprof.Profile(c.Response, c.Request)
	})

	r.POST(base+"/symbol", func(c *router.Context) {
		pprof.Symbol(c.Response, c.Request)
	})

	r.GET(base+"/symbol", func(c *router.Context) {
		pprof.Symbol(c.Response, c.Request)
	})

	r.GET(base+"/trace", func(c *router.Context) {
		pprof.Trace(c.Response, c.Request)
	})

	// Named profiles
	profiles := []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"}
	for _, p := range profiles {
		profileName := p // Capture for closure
		r.GET(base+"/"+profileName, func(c *router.Context) {
			pprof.Handler(profileName).ServeHTTP(c.Response, c.Request)
		})
	}
}
