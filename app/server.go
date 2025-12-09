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
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"rivaas.dev/router"
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
		_ = a.logging.FlushBuffer() // Ignore errors, logging is best-effort
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
	// Start metrics server if configured
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
// Unlike stdlib's http.Server which uses separate Shutdown() call, this method combines
// serving and lifecycle management for a simpler API. Users should pass a context
// configured with signal.NotifyContext for graceful shutdown on OS signals.
func (a *App) runServer(ctx context.Context, server *http.Server, startFunc serverStartFunc, protocol string) error {
	// Start server in a goroutine
	serverErr := make(chan error, 1)
	serverReady := make(chan struct{})
	go func() {
		a.printStartupBanner(server.Addr, protocol)

		// Flush any buffered startup logs after the banner is printed.
		// This ensures all initialization logs appear after the banner for cleaner DX.
		a.flushStartupLogs()

		a.logStartupInfo(ctx, server.Addr, protocol)
		// Routes are now displayed as part of the startup banner

		// Signal that server is ready to accept connections
		close(serverReady)

		if err := startFunc(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("%s server failed to start: %w", protocol, err)
		}
	}()

	// Wait for server to be ready, then execute OnReady hooks
	<-serverReady
	a.executeReadyHooks()

	// Wait for context cancellation or server error
	// Note: Signal handling should be done by the caller via signal.NotifyContext
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		a.logLifecycleEvent(ctx, slog.LevelInfo, "server shutting down", "protocol", protocol, "reason", ctx.Err())
	}

	// Create a deadline for shutdown.
	// We use context.Background() because the original ctx is already canceled (that's what triggered
	// the shutdown). Using a canceled context as parent would give us 0 time for graceful shutdown.
	// This is the standard Go pattern - the parent ctx signals WHEN to shutdown, the fresh context
	// controls HOW LONG the shutdown can take.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.config.server.shutdownTimeout)
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
	a.executeStopHooks()

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
		specJSON, etag, err := a.openapi.GenerateSpec()
		if err != nil {
			if writeErr := c.Stringf(http.StatusInternalServerError, "Failed to generate OpenAPI specification: %v", err); writeErr != nil {
				c.Logger().Error("failed to write error response", "err", writeErr)
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
		c.Response.Write(specJSON)
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
	<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.30.2/swagger-ui.css" />
</head>
<body>
	<div id="swagger-ui"></div>
	<script src="https://unpkg.com/swagger-ui-dist@5.30.2/swagger-ui-bundle.js" crossorigin></script>
	<script>
		window.onload = () => {
			window.ui = SwaggerUIBundle(` + configJSON + `);
		};
	</script>
</body>
</html>`

			if htmlErr := c.HTML(http.StatusOK, html); htmlErr != nil {
				c.Logger().Error("failed to write HTML response", "err", htmlErr)
			}
		})
	}
}

// Start starts the HTTP server with graceful shutdown.
// Start automatically freezes the router before starting, making routes immutable.
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
//	if err := app.Start(ctx, ":8080"); err != nil {
//	    log.Fatal(err)
//	}
func (a *App) Start(ctx context.Context, addr string) error {
	// Start observability servers (metrics, etc.)
	if err := a.startObservability(ctx); err != nil {
		return fmt.Errorf("failed to start observability: %w", err)
	}

	// Execute OnStart hooks sequentially, stopping on first error
	if err := a.executeStartHooks(ctx); err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}

	// Register OpenAPI endpoints before freezing
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

	return a.runServer(ctx, server, server.ListenAndServe, "HTTP")
}

// StartTLS starts the HTTPS server with graceful shutdown.
// StartTLS automatically freezes the router before starting, making routes immutable.
//
// The context controls the application lifecycle - when canceled, it triggers
// graceful shutdown of the server and all observability components (metrics, tracing).
//
// Note: Signal handling should be configured by the caller using signal.NotifyContext.
//
// Example:
//
//	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
//	defer cancel()
//
//	if err := app.StartTLS(ctx, ":8443", "server.crt", "server.key"); err != nil {
//	    log.Fatal(err)
//	}
func (a *App) StartTLS(ctx context.Context, addr, certFile, keyFile string) error {
	// Start observability servers (metrics, etc.)
	if err := a.startObservability(ctx); err != nil {
		return fmt.Errorf("failed to start observability: %w", err)
	}

	// Execute OnStart hooks sequentially, stopping on first error
	if err := a.executeStartHooks(ctx); err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}

	// Register OpenAPI endpoints before freezing
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

	return a.runServer(ctx, server, func() error {
		return server.ListenAndServeTLS(certFile, keyFile)
	}, "HTTPS")
}

// StartMTLS starts an HTTPS server with mutual TLS (mTLS) authentication.
// It requires both client and server certificates for bidirectional authentication.
// It automatically freezes the router before starting, making routes immutable.
//
// The context controls the application lifecycle - when canceled, it triggers
// graceful shutdown of the server and all observability components (metrics, tracing).
//
// Note: Signal handling should be configured by the caller using signal.NotifyContext.
//
// It configures the server to:
//   - Require client certificates (ClientAuth: RequireAndVerifyClientCert)
//   - Validate client certificates against ClientCAs
//   - Optionally authorize clients using the WithAuthorize callback
//   - Support SNI via WithSNI callback
//   - Support hot-reload via WithConfigForClient callback
//
// Example:
//
//	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
//	defer cancel()
//
//	// Load server certificate
//	serverCert, err := tls.LoadX509KeyPair("server.crt", "server.key")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Load CA certificate for client validation
//	caCert, err := os.ReadFile("ca.crt")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	caCertPool := x509.NewCertPool()
//	caCertPool.AppendCertsFromPEM(caCert)
//
//	// Start server with mTLS
//	err = app.StartMTLS(ctx, ":8443", serverCert,
//	    app.WithClientCAs(caCertPool),
//	    app.WithMinVersion(tls.VersionTLS13),
//	    app.WithAuthorize(func(cert *x509.Certificate) (string, bool) {
//	        // Extract principal from certificate
//	        return cert.Subject.CommonName, cert.Subject.CommonName != ""
//	    }),
//	)
func (a *App) StartMTLS(ctx context.Context, addr string, serverCert tls.Certificate, opts ...MTLSOption) error {
	// Create mTLS configuration from options
	cfg := newMTLSConfig(serverCert, opts...)

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return fmt.Errorf("invalid mTLS configuration: %w", err)
	}

	// Start observability servers (metrics, etc.)
	if err := a.startObservability(ctx); err != nil {
		return fmt.Errorf("failed to start observability: %w", err)
	}

	// Execute OnStart hooks sequentially, stopping on first error
	if err := a.executeStartHooks(ctx); err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}

	// Register OpenAPI endpoints before freezing
	a.registerOpenAPIEndpoints()

	// Freeze router before starting (point of no return)
	a.router.Freeze()

	// Build TLS configuration
	tlsConfig := cfg.buildTLSConfig()

	// Create listener
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Wrap listener with TLS
	tlsListener := tls.NewListener(listener, tlsConfig)

	// Create HTTP server
	server := &http.Server{
		Addr:              addr,
		Handler:           a.router,
		TLSConfig:         tlsConfig,
		ReadTimeout:       a.config.server.readTimeout,
		WriteTimeout:      a.config.server.writeTimeout,
		IdleTimeout:       a.config.server.idleTimeout,
		ReadHeaderTimeout: a.config.server.readHeaderTimeout,
		MaxHeaderBytes:    a.config.server.maxHeaderBytes,
	}

	// Wrap ConnState callback to authorize client certificates
	// Authorization happens at connection time; principal extraction happens per-request
	originalConnState := server.ConnState
	server.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateActive && !authorizeMTLSConnection(conn, cfg) {
			conn.Close()

			return
		}

		// Call original ConnState if set
		if originalConnState != nil {
			originalConnState(conn, state)
		}
	}

	// Use runServer helper with custom start function for TLS listener
	return a.runServer(ctx, server, func() error {
		return server.Serve(tlsListener)
	}, "mTLS")
}

// authorizeMTLSConnection checks if the TLS connection is authorized.
// Returns true if authorized (or no authorization required), false if denied.
func authorizeMTLSConnection(conn net.Conn, cfg *mtlsConfig) bool {
	// No authorize callback means all connections are allowed
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
