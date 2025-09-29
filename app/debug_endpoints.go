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
)

// DebugEndpointsOpts configures debug endpoints like pprof.
type DebugEndpointsOpts struct {
	// MountPrefix is the prefix under which to mount debug endpoints.
	// Defaults to "/debug" if not specified.
	// Example: "/debug" mounts pprof at "/debug/pprof"
	MountPrefix string

	// EnablePprof determines if pprof endpoints should be registered.
	// pprof endpoints are only registered if this is true.
	// WARNING: pprof exposes sensitive runtime information. Only enable in development
	// or behind authentication/authorization.
	EnablePprof bool
}

// WithDebugEndpoints registers debug endpoints like pprof.
//
// Security rationale: pprof endpoints are disabled by default and require explicit
// opt-in because they expose sensitive runtime information that can be exploited:
//
// Attack vectors:
//   - Goroutine dumps reveal internal logic and potential race conditions
//   - Heap dumps may contain secrets, tokens, or PII in memory
//   - CPU profiling can be used for timing attacks or DoS (profiling has overhead)
//   - Alloc profiles reveal memory usage patterns useful for resource exhaustion attacks
//
// Safe usage patterns:
//  1. Development: Enable unconditionally (no external exposure)
//  2. Staging: Enable behind VPN or IP allowlist
//  3. Production: Enable only with proper authentication middleware
//     Example: app.Use(authMiddleware); app.WithDebugEndpoints(...)
//
// Endpoints registered (if EnablePprof is true):
//   - GET /debug/pprof/ - Main pprof index
//   - GET /debug/pprof/cmdline - Command line
//   - GET /debug/pprof/profile - CPU profile
//   - GET /debug/pprof/symbol - Symbol lookup
//   - POST /debug/pprof/symbol - Symbol lookup
//   - GET /debug/pprof/trace - Execution trace
//   - GET /debug/pprof/{profile} - Named profiles (allocs, block, goroutine, heap, mutex, threadcreate)
//
// WithDebugEndpoints returns an error if any endpoint path already exists (collision detection).
//
// Example:
//
//	_ = a.WithDebugEndpoints(app.DebugEndpointsOpts{
//	    MountPrefix: "/debug",
//	    EnablePprof: os.Getenv("PPROF_ENABLED") == "true",
//	})
func (a *App) WithDebugEndpoints(o DebugEndpointsOpts) error {
	if !o.EnablePprof {
		return nil // Silently skip if disabled
	}

	prefix := o.MountPrefix
	if prefix == "" {
		prefix = "/debug"
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
		r.GET(base+"/"+p, func(c *router.Context) {
			pprof.Handler(p).ServeHTTP(c.Response, c.Request)
		})
	}
}
