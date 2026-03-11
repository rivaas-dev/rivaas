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
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"

	"rivaas.dev/router"
	"rivaas.dev/router/route"
)

// serverStartFunc defines the function type for starting a server.
type serverStartFunc func() error

// logLifecycleEvent logs a lifecycle event using the base logger.
func (a *App) logLifecycleEvent(ctx context.Context, level slog.Level, msg string, args ...any) {
	logger := a.BaseLogger()
	if logger.Enabled(ctx, level) {
		logger.Log(ctx, level, msg, args...)
	}
}

// flushStartupLogs flushes any buffered startup logs.
// This is called after the banner is printed to ensure clean terminal output.
func (a *App) flushStartupLogs() {
	if a.logging != nil {
		//nolint:errcheck // Best-effort flush; failure here doesn't affect server startup
		a.logging.FlushBuffer()
	}
}

// logStartupInfo logs startup information including address, environment, and observability.
func (a *App) logStartupInfo(ctx context.Context, addr, protocol string) {
	attrs := []any{
		"address", addr,
		"environment", a.config.environment,
		"protocol", protocol,
	}

	if a.metrics != nil {
		attrs = append(attrs, "metrics_enabled", true, "metrics_address", a.metrics.ServerAddress())
	}

	a.logLifecycleEvent(ctx, slog.LevelInfo, "server starting", attrs...)

	if a.tracing != nil {
		a.logLifecycleEvent(ctx, slog.LevelInfo, "tracing enabled")
	}
}

// startObservability starts observability components (metrics, tracing) with the given context.
// The context is used for network connections and server lifecycle.
func (a *App) startObservability(ctx context.Context) error {
	// Start a metrics server if configured
	if a.metrics != nil {
		if err := a.metrics.Start(ctx); err != nil {
			return fmt.Errorf("failed to start metrics server: %w", err)
		}
	}

	// Start tracing if configured (initializes OTLP exporters)
	if a.tracing != nil {
		if err := a.tracing.Start(ctx); err != nil {
			return fmt.Errorf("failed to start tracing: %w", err)
		}
	}

	return nil
}

func (a *App) shutdownObservability(ctx context.Context) {
	// Shutdown metrics if running
	if a.metrics != nil {
		if err := a.metrics.Shutdown(ctx); err != nil {
			a.logLifecycleEvent(ctx, slog.LevelWarn, "metrics shutdown failed", "error", err)
		}
	}

	// Shutdown tracing if running
	if a.tracing != nil {
		if err := a.tracing.Shutdown(ctx); err != nil {
			a.logLifecycleEvent(ctx, slog.LevelWarn, "tracing shutdown failed", "error", err)
		}
	}
}

// runServer handles the common lifecycle for starting and shutting down an HTTP server.
// It is used by [App.Start], [App.StartTLS], and [App.StartMTLS].
// The context controls the server lifecycle - when canceled, it triggers graceful shutdown.
//
// Unlike stdlib's http.Server, which uses separate Shutdown() call, this method combines
// serving and lifecycle management for a simpler API. Users should pass a context
// configured with signal.NotifyContext for graceful shutdown on OS signals.
func (a *App) runServer(ctx context.Context, server *http.Server, startFunc serverStartFunc, protocol string) error {
	// Start a server in a goroutine
	serverErr := make(chan error, 1)
	serverReady := make(chan struct{})
	go func() {
		a.printStartupBanner(server.Addr, protocol)

		// Flush any buffered startup logs after the banner is printed.
		// This ensures all initialization logs appear after the banner for cleaner DX.
		a.flushStartupLogs()

		a.logStartupInfo(ctx, server.Addr, protocol)
		// Routes are now displayed as part of the startup banner

		// Signal that the server is ready to accept connections
		close(serverReady)

		if err := startFunc(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("%s server failed to start: %w", protocol, err)
		}
	}()

	// Wait for the server to be ready, then execute OnReady hooks
	<-serverReady
	a.executeReadyHooks(ctx)

	// Set up SIGHUP: handle reload when hooks exist, otherwise ignore so the process isn't killed
	var sighupCh <-chan os.Signal
	if a.hasReloadHooks() {
		ch, cleanup := setupReloadSignal()
		defer cleanup()
		sighupCh = ch
	} else {
		ignoreReloadSignal() // Unix: SIGHUP ignored so the process isn't killed
	}

	// Event loop: wait for shutdown, reload, or server error
	// When sighupCh is nil (no reload hooks registered), the nil channel case blocks forever with zero overhead
	for {
		select {
		case err := <-serverErr:
			return err

		case <-sighupCh:
			a.logLifecycleEvent(ctx, slog.LevelInfo, "reload signal received", "signal", "SIGHUP")
			if err := a.Reload(ctx); err != nil {
				// Error already logged inside Reload(), continue serving
				_ = err
			}

		case <-ctx.Done():
			a.logLifecycleEvent(ctx, slog.LevelInfo, "server shutting down", "protocol", protocol, "reason", ctx.Err())
			goto shutdown
		}
	}

shutdown:
	// Create a deadline for shutdown.
	// We use context.WithoutCancel() to preserve context values (tracing, logging) while ignoring
	// the parent's cancellation. The parent ctx is already canceled (that's what triggered shutdown),
	// but we want to keep its values for observability during shutdown operations.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.config.server.shutdownTimeout)
	defer cancel()

	// Execute OnShutdown hooks (LIFO order)
	a.executeShutdownHooks(shutdownCtx)

	// Shutdown the server
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("%s server forced to shutdown: %w", protocol, err)
	}

	// Shutdown observability components (metrics and tracing)
	a.shutdownObservability(shutdownCtx)

	// Execute OnStop hooks (best-effort)
	a.executeStopHooks(shutdownCtx)

	a.logLifecycleEvent(shutdownCtx, slog.LevelInfo, "server exited", "protocol", protocol)

	return nil
}

// registerOpenAPIEndpoints registers OpenAPI spec and UI endpoints.
// registerOpenAPIEndpoints is the integration between router and openapi packages.
func (a *App) registerOpenAPIEndpoints() {
	if a.openapi == nil {
		return
	}

	// Register spec endpoint
	a.router.GET(a.openapi.SpecPath(), func(c *router.Context) {
		specJSON, etag, err := a.openapi.GenerateSpec(c.Request.Context())
		if err != nil {
			if writeErr := c.Stringf(http.StatusInternalServerError, "Failed to generate OpenAPI specification: %v", err); writeErr != nil {
				slog.ErrorContext(c.RequestContext(), "failed to write error response", "err", writeErr)
			}

			return
		}

		// Check If-None-Match header for caching
		if match := c.Request.Header.Get("If-None-Match"); match != "" && match == etag {
			c.Status(http.StatusNotModified)
			return
		}

		c.Response.Header().Set("ETag", etag)
		c.Response.Header().Set("Cache-Control", "public, max-age=3600")
		c.Response.Header().Set("Content-Type", "application/json")
		if _, err = c.Response.Write(specJSON); err != nil { //nolint:gosec // G705: specJSON is server-generated OpenAPI spec, not user input
			slog.ErrorContext(c.RequestContext(), "failed to write spec response", "err", err)
		}
	})

	// Update route info to show the builtin handler name
	a.router.UpdateRouteInfo("GET", a.openapi.SpecPath(), "", func(info *route.Info) {
		info.HandlerName = "[builtin] openapi-spec"
	})

	// Register UI endpoint if enabled
	if a.openapi.ServeUI() {
		a.router.GET(a.openapi.UIPath(), func(c *router.Context) {
			ui := a.openapi.UIConfig()
			configJSON, err := ui.ToJSON(a.openapi.SpecPath())
			if err != nil {
				// Fallback to basic config
				configJSON = `{"url":"` + a.openapi.SpecPath() + `","dom_id":"#swagger-ui"}`
			}

			html := `<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="utf-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />
	<meta name="description" content="API Documentation" />
	<title>API Documentation</title>
	<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.32.0/swagger-ui.css" />
</head>
<body>
	<div id="swagger-ui"></div>
	<script src="https://unpkg.com/swagger-ui-dist@5.32.0/swagger-ui-bundle.js" crossorigin></script>
	<script>
		window.onload = () => {
			window.ui = SwaggerUIBundle(` + configJSON + `);
		};
	</script>
</body>
</html>`

			if htmlErr := c.HTML(http.StatusOK, html); htmlErr != nil {
				slog.ErrorContext(c.RequestContext(), "failed to write HTML response", "err", htmlErr)
			}
		})

		// Update route info to show the builtin handler name
		a.router.UpdateRouteInfo("GET", a.openapi.UIPath(), "", func(info *route.Info) {
			info.HandlerName = "[builtin] openapi-ui"
		})
	}
}

// Start starts the server with graceful shutdown.
// Start automatically freezes the router before starting, making routes immutable.
// The server runs HTTP, HTTPS, or mTLS depending on configuration: use [WithTLS] or
// [WithMTLS] at construction to serve over TLS; otherwise plain HTTP is used.
//
// The server listens on the address configured via [WithPort] and [WithHost].
// Default is :8080 for HTTP and :8443 when using [WithTLS] or [WithMTLS], overridable by
// [WithPort] and by RIVAAS_PORT and RIVAAS_HOST when [WithEnv] is used.
//
// The context controls the application lifecycle - when canceled, it triggers
// graceful shutdown of the server and all observability components (metrics, tracing).
//
// Note: Signal handling should be configured by the caller using signal.NotifyContext.
// This follows the Go pattern of explicit signal handling at the application boundary.
//
// Example:
//
//	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
//	defer cancel()
//
//	app := app.MustNew(
//	    app.WithServiceName("my-service"),
//	    app.WithPort(3000),
//	)
//	if err := app.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// HTTPS example (default port 8443):
//
//	app := app.MustNew(
//	    app.WithServiceName("my-service"),
//	    app.WithTLS("server.crt", "server.key"),
//	)
//	if err := app.Start(ctx); err != nil { ... }
func (a *App) Start(ctx context.Context) error {
	addr := a.config.server.ListenAddr()

	// Start observability servers (metrics, etc.)
	if err := a.startObservability(ctx); err != nil {
		return fmt.Errorf("failed to start observability: %w", err)
	}

	// Execute OnStart hooks sequentially, stopping on first error
	if err := a.executeStartHooks(ctx); err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}

	// Register OpenAPI endpoints before freezing
	//nolint:contextcheck // Handler registration - context comes from request at runtime
	a.registerOpenAPIEndpoints()

	// Freeze router before starting (point of no return)
	a.router.Freeze()

	server := &http.Server{
		Addr:              addr,
		Handler:           a.router,
		ReadTimeout:       a.config.server.readTimeout,
		WriteTimeout:      a.config.server.writeTimeout,
		IdleTimeout:       a.config.server.idleTimeout,
		ReadHeaderTimeout: a.config.server.readHeaderTimeout,
		MaxHeaderBytes:    a.config.server.maxHeaderBytes,
	}

	// Branch on transport: TLS (HTTPS), mTLS, or plain HTTP
	if a.config.server.tlsCertFile != "" {
		return a.runServer(ctx, server, func() error {
			return server.ListenAndServeTLS(a.config.server.tlsCertFile, a.config.server.tlsKeyFile)
		}, "HTTPS")
	}
	if len(a.config.server.mtlsServerCert.Certificate) > 0 {
		return a.startMTLS(ctx, server, addr)
	}
	return a.runServer(ctx, server, server.ListenAndServe, "HTTP")
}

// startMTLS runs the server with mTLS using config from a.config.server.
func (a *App) startMTLS(ctx context.Context, server *http.Server, addr string) error {
	cfg := newMTLSConfig(a.config.server.mtlsServerCert, a.config.server.mtlsOpts...)
	tlsConfig := cfg.buildTLSConfig()

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	tlsListener := tls.NewListener(listener, tlsConfig)

	server.TLSConfig = tlsConfig
	originalConnState := server.ConnState
	server.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateActive && !authorizeMTLSConnection(conn, cfg) {
			if closeErr := conn.Close(); closeErr != nil {
				a.logLifecycleEvent(ctx, slog.LevelError, "failed to close unauthorized mTLS connection", "error", closeErr)
			}
			return
		}
		if originalConnState != nil {
			originalConnState(conn, state)
		}
	}

	return a.runServer(ctx, server, func() error {
		return server.Serve(tlsListener)
	}, "mTLS")
}

// authorizeMTLSConnection checks if the TLS connection is authorized.
// Returns true if authorized (or no authorization required), false if denied.
func authorizeMTLSConnection(conn net.Conn, cfg *mtlsConfig) bool {
	// No authorized callback means all connections are allowed
	if cfg.authorize == nil {
		return true
	}

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return true // Not a TLS connection, skip authorization
	}

	connState := tlsConn.ConnectionState()
	if len(connState.PeerCertificates) == 0 {
		return true // No peer certificate, skip authorization
	}

	_, allowed := cfg.authorize(connState.PeerCertificates[0])

	return allowed
}
