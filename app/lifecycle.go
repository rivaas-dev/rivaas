// Package app provides the main application implementation for Rivaas.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"rivaas.dev/router"
)

// Hooks manages application lifecycle hooks.
type Hooks struct {
	onStart    []func(context.Context) error // Sequential, fail-fast
	onReady    []func()                      // Async OK
	onShutdown []func(context.Context)       // LIFO order
	onStop     []func()                      // Best effort
	onRoute    []func(router.Route)          // Fire during registration
	mu         sync.Mutex                    // Protects hook slices
}

// OnStart registers a hook that runs before the server starts listening.
// Hooks run sequentially and if any returns an error, startup is aborted.
// Use this for initialization that must succeed (database connections, migrations, etc.).
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
// Use this for warmup tasks, service discovery registration, etc.
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

// OnShutdown registers a hook that runs during graceful shutdown.
// Hooks run in reverse order (LIFO) and receive a context with the shutdown timeout.
// Use this for cleanup that must complete within the timeout (closing connections, flushing buffers).
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
// Use this for final cleanup that doesn't need to complete within a timeout.
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
// Useful for route validation, logging, or documentation generation.
// Disabled after router is frozen.
//
// Example:
//
//	app.OnRoute(func(route router.Route) {
//	    log.Printf("Registered: %s %s", route.Method(), route.Path())
//	})
func (a *App) OnRoute(fn func(router.Route)) {
	if a.router.Frozen() {
		panic("cannot register hooks after router is frozen")
	}
	a.hooks.mu.Lock()
	defer a.hooks.mu.Unlock()
	a.hooks.onRoute = append(a.hooks.onRoute, fn)
}

// fireRouteHook fires all OnRoute hooks for a newly registered route.
// Only fires if router is not frozen (hooks disabled after freeze).
func (a *App) fireRouteHook(route router.Route) {
	if a.router.Frozen() {
		return // Hooks disabled after freeze
	}

	a.hooks.mu.Lock()
	hooks := make([]func(router.Route), len(a.hooks.onRoute))
	copy(hooks, a.hooks.onRoute)
	a.hooks.mu.Unlock()

	for _, hook := range hooks {
		hook(route)
	}
}

// executeStartHooks runs all OnStart hooks sequentially.
// Returns an error if any hook fails.
func (a *App) executeStartHooks(ctx context.Context) error {
	a.hooks.mu.Lock()
	hooks := make([]func(context.Context) error, len(a.hooks.onStart))
	copy(hooks, a.hooks.onStart)
	a.hooks.mu.Unlock()

	for i, hook := range hooks {
		if err := hook(ctx); err != nil {
			return fmt.Errorf("OnStart hook %d failed: %w", i, err)
		}
	}
	return nil
}

// executeReadyHooks runs all OnReady hooks asynchronously.
func (a *App) executeReadyHooks() {
	a.hooks.mu.Lock()
	hooks := make([]func(), len(a.hooks.onReady))
	copy(hooks, a.hooks.onReady)
	a.hooks.mu.Unlock()

	for _, hook := range hooks {
		go hook() // Fire and forget
	}
}

// executeShutdownHooks runs all OnShutdown hooks in reverse order (LIFO).
func (a *App) executeShutdownHooks(ctx context.Context) {
	a.hooks.mu.Lock()
	hooks := make([]func(context.Context), len(a.hooks.onShutdown))
	copy(hooks, a.hooks.onShutdown)
	a.hooks.mu.Unlock()

	// Execute in reverse order (LIFO)
	for i := len(hooks) - 1; i >= 0; i-- {
		hooks[i](ctx)
	}
}

// executeStopHooks runs all OnStop hooks in best-effort mode.
func (a *App) executeStopHooks() {
	a.hooks.mu.Lock()
	hooks := make([]func(), len(a.hooks.onStop))
	copy(hooks, a.hooks.onStop)
	a.hooks.mu.Unlock()

	for _, hook := range hooks {
		func() {
			defer func() {
				if r := recover(); r != nil {
					a.logLifecycleEvent(slog.LevelWarn, "OnStop hook panic", "error", r)
				}
			}()
			hook()
		}()
	}
}
