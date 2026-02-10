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
	"fmt"
	"log/slog"
	"sync"

	"rivaas.dev/router/route"
)

// Hooks manages application lifecycle hooks.
// It stores callbacks for different lifecycle events.
type Hooks struct {
	onStart    []func(context.Context) error // Sequential, stops on first error
	onReady    []func()                      // Async OK
	onReload   []func(context.Context) error // Sequential, stops on first error
	onShutdown []func(context.Context)       // LIFO order
	onStop     []func()                      // Best effort
	onRoute    []func(*route.Route)          // Fire during registration
	mu         sync.Mutex                    // Protects hook slices
}

// OnStart registers a hook that runs before the server starts listening.
// Hooks run sequentially, and if any hook returns an error, startup is aborted.
// It should be used for initialization that must succeed (database connections, migrations, etc.).
//
// Example:
//
//	app.OnStart(func(ctx context.Context) error {
//	    return db.PingContext(ctx)
//	})
func (a *App) OnStart(fn func(context.Context) error) {
	if a.router.Frozen() {
		panic("cannot register hooks after router is frozen")
	}
	a.hooks.mu.Lock()
	defer a.hooks.mu.Unlock()
	a.hooks.onStart = append(a.hooks.onStart, fn)
}

// OnReady registers a hook that runs after the server starts listening.
// Hooks can run asynchronously and errors are logged but don't stop the server.
// It should be used for warmup tasks, service discovery registration, etc.
//
// Example:
//
//	app.OnReady(func() {
//	    log.Println("Server ready!")
//	    registerWithConsul()
//	})
func (a *App) OnReady(fn func()) {
	if a.router.Frozen() {
		panic("cannot register hooks after router is frozen")
	}
	a.hooks.mu.Lock()
	defer a.hooks.mu.Unlock()
	a.hooks.onReady = append(a.hooks.onReady, fn)
}

// OnReload registers a hook that runs when the application receives a reload signal (SIGHUP)
// or when Reload() is called programmatically.
// Hooks run sequentially, and if any hook returns an error, subsequent hooks are skipped.
// Reload errors are logged but do not stop the server - it continues serving with the old configuration.
//
// OnReload should be used for reloading runtime configuration without restarting the server:
//   - Re-reading configuration files
//   - Rotating TLS certificates
//   - Flushing caches
//   - Adjusting log levels
//   - Updating connection pool settings
//
// SIGHUP signal handling is automatically enabled when at least one OnReload hook is registered.
// On Unix systems, sending SIGHUP to the process will trigger all registered reload hooks.
// On Windows, SIGHUP is not available, but Reload() can still be called programmatically.
//
// Note: Routes and middleware cannot be reloaded as the router is frozen after startup.
//
// Example:
//
//	app.OnReload(func(ctx context.Context) error {
//	    cfg, err := loadConfig("config.yaml")
//	    if err != nil {
//	        return fmt.Errorf("failed to load config: %w", err)
//	    }
//	    applyConfig(cfg)
//	    return nil
//	})
func (a *App) OnReload(fn func(context.Context) error) {
	if a.router.Frozen() {
		panic("cannot register hooks after router is frozen")
	}
	a.hooks.mu.Lock()
	defer a.hooks.mu.Unlock()
	a.hooks.onReload = append(a.hooks.onReload, fn)
}

// OnShutdown registers a hook that runs during graceful shutdown.
// Hooks run in reverse order (LIFO) and receive a context with the shutdown timeout.
// It should be used for cleanup that must complete within the timeout (closing connections, flushing buffers).
//
// Example:
//
//	app.OnShutdown(func(ctx context.Context) {
//	    db.Close()
//	    flushMetrics(ctx)
//	})
func (a *App) OnShutdown(fn func(context.Context)) {
	if a.router.Frozen() {
		panic("cannot register hooks after router is frozen")
	}
	a.hooks.mu.Lock()
	defer a.hooks.mu.Unlock()
	a.hooks.onShutdown = append(a.hooks.onShutdown, fn)
}

// OnStop registers a hook that runs after the server stops.
// Hooks run in best-effort mode - panics are caught and logged.
// It should be used for final cleanup that doesn't need to complete within a timeout.
//
// Example:
//
//	app.OnStop(func() {
//	    cleanupTempFiles()
//	})
func (a *App) OnStop(fn func()) {
	if a.router.Frozen() {
		panic("cannot register hooks after router is frozen")
	}
	a.hooks.mu.Lock()
	defer a.hooks.mu.Unlock()
	a.hooks.onStop = append(a.hooks.onStop, fn)
}

// OnRoute registers a hook that fires when a route is registered.
// OnRoute is useful for route validation, logging, or documentation generation.
// OnRoute hooks are disabled after router is frozen.
//
// Example:
//
//	app.OnRoute(func(rt *route.Route) {
//	    log.Printf("Registered: %s %s", rt.Method(), rt.Path())
//	})
func (a *App) OnRoute(fn func(*route.Route)) {
	if a.router.Frozen() {
		panic("cannot register hooks after router is frozen")
	}
	a.hooks.mu.Lock()
	defer a.hooks.mu.Unlock()
	a.hooks.onRoute = append(a.hooks.onRoute, fn)
}

// fireRouteHook fires all OnRoute hooks for a newly registered route.
// fireRouteHook only fires if router is not frozen (hooks disabled after freeze).
func (a *App) fireRouteHook(rt *route.Route) {
	if a.router.Frozen() {
		return // Hooks disabled after freeze
	}

	a.hooks.mu.Lock()
	hooks := make([]func(*route.Route), 0, len(a.hooks.onRoute))
	hooks = append(hooks, a.hooks.onRoute...)
	a.hooks.mu.Unlock()

	for _, hook := range hooks {
		hook(rt)
	}
}

// executeStartHooks runs all OnStart hooks sequentially.
// It returns an error if any hook fails.
func (a *App) executeStartHooks(ctx context.Context) error {
	a.hooks.mu.Lock()
	hooks := make([]func(context.Context) error, 0, len(a.hooks.onStart))
	hooks = append(hooks, a.hooks.onStart...)
	a.hooks.mu.Unlock()

	for i, hook := range hooks {
		if err := hook(ctx); err != nil {
			return fmt.Errorf("OnStart hook %d failed: %w", i, err)
		}
	}

	return nil
}

// executeReadyHooks runs all OnReady hooks asynchronously.
// It runs hooks in fire-and-forget mode with panic recovery to prevent silent failures.
func (a *App) executeReadyHooks(ctx context.Context) {
	a.hooks.mu.Lock()
	hooks := make([]func(), 0, len(a.hooks.onReady))
	hooks = append(hooks, a.hooks.onReady...)
	a.hooks.mu.Unlock()

	for _, hook := range hooks {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Use context.Background() because OnReady hooks are fire-and-forget goroutines
					// that don't receive a context. During panic recovery, we just need to log the error.
					a.logLifecycleEvent(ctx, slog.LevelError, "OnReady hook panic", "error", r)
				}
			}()
			hook()
		}()
	}
}

// executeReloadHooks runs all OnReload hooks sequentially.
// It returns an error if any hook fails, stopping execution of subsequent hooks.
func (a *App) executeReloadHooks(ctx context.Context) error {
	a.hooks.mu.Lock()
	hooks := make([]func(context.Context) error, 0, len(a.hooks.onReload))
	hooks = append(hooks, a.hooks.onReload...)
	a.hooks.mu.Unlock()

	for i, hook := range hooks {
		if err := hook(ctx); err != nil {
			return fmt.Errorf("OnReload hook %d failed: %w", i, err)
		}
	}

	return nil
}

// executeShutdownHooks runs all OnShutdown hooks in reverse order (LIFO).
func (a *App) executeShutdownHooks(ctx context.Context) {
	a.hooks.mu.Lock()
	hooks := make([]func(context.Context), 0, len(a.hooks.onShutdown))
	hooks = append(hooks, a.hooks.onShutdown...)
	a.hooks.mu.Unlock()

	// Execute in reverse order (LIFO)
	for i := len(hooks) - 1; i >= 0; i-- {
		hooks[i](ctx)
	}
}

// executeStopHooks runs all OnStop hooks in best-effort mode.
func (a *App) executeStopHooks(ctx context.Context) {
	a.hooks.mu.Lock()
	hooks := make([]func(), 0, len(a.hooks.onStop))
	hooks = append(hooks, a.hooks.onStop...)
	a.hooks.mu.Unlock()

	for _, hook := range hooks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					a.logLifecycleEvent(ctx, slog.LevelWarn, "OnStop hook panic", "error", r)
				}
			}()
			hook()
		}()
	}
}
