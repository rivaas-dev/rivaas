// Package app provides the main application implementation for Rivaas.
package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// serverStartFunc defines the function type for starting a server.
type serverStartFunc func() error

// logLifecycleEvent logs a lifecycle event using structured logging if available,
// otherwise falls back to the standard library log package.
func (a *App) logLifecycleEvent(level slog.Level, msg string, args ...any) {
	if a.logging != nil {
		logger := a.logging.Logger()
		if logger.Enabled(context.Background(), level) {
			logger.Log(context.Background(), level, msg, args...)
		}
	} else {
		// Fall back to stdlib log for backwards compatibility
		if len(args) == 0 {
			log.Println(msg)
		} else {
			// Format key-value pairs for stdlib log
			logMsg := msg
			for i := 0; i < len(args)-1; i += 2 {
				if key, ok := args[i].(string); ok {
					logMsg += fmt.Sprintf(" %s=%v", key, args[i+1])
				}
			}
			log.Println(logMsg)
		}
	}
}

// logStartupInfo logs startup information including address, environment, and observability status.
func (a *App) logStartupInfo(addr, protocol string) {
	attrs := []any{
		"address", addr,
		"environment", a.config.environment,
		"protocol", protocol,
	}

	if a.metrics != nil {
		attrs = append(attrs, "metrics_enabled", true, "metrics_address", a.metrics.GetServerAddress())
	}

	a.logLifecycleEvent(slog.LevelInfo, "server starting", attrs...)

	if a.tracing != nil {
		a.logLifecycleEvent(slog.LevelInfo, "tracing enabled")
	}
}

// shutdownObservability gracefully shuts down all enabled observability components.
func (a *App) shutdownObservability(ctx context.Context) {
	// Shutdown metrics if running
	if a.metrics != nil {
		if err := a.metrics.Shutdown(ctx); err != nil {
			a.logLifecycleEvent(slog.LevelWarn, "metrics shutdown failed", "error", err)
		}
	}

	// Shutdown tracing if running
	if a.tracing != nil {
		if err := a.tracing.Shutdown(ctx); err != nil {
			a.logLifecycleEvent(slog.LevelWarn, "tracing shutdown failed", "error", err)
		}
	}
}

// runServer handles the common lifecycle logic for starting and shutting down an HTTP server.
// It accepts an http.Server and a startFunc (either ListenAndServe or ListenAndServeTLS).
func (a *App) runServer(server *http.Server, startFunc serverStartFunc, protocol string) error {
	// Start server in a goroutine
	serverErr := make(chan error, 1)
	serverReady := make(chan struct{})
	go func() {
		a.printStartupBanner(server.Addr, protocol)
		a.logStartupInfo(server.Addr, protocol)
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

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case <-quit:
		a.logLifecycleEvent(slog.LevelInfo, "server shutting down", "protocol", protocol)
	}

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), a.config.server.shutdownTimeout)
	defer cancel()

	// Execute OnShutdown hooks (LIFO order)
	a.executeShutdownHooks(ctx)

	// Shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("%s server forced to shutdown: %w", protocol, err)
	}

	// Shutdown observability components (metrics and tracing)
	a.shutdownObservability(ctx)

	// Execute OnStop hooks (best-effort)
	a.executeStopHooks()

	a.logLifecycleEvent(slog.LevelInfo, "server exited", "protocol", protocol)
	return nil
}

// Run starts the HTTP server with graceful shutdown.
// This automatically freezes the router before starting, making routes immutable.
func (a *App) Run(addr string) error {
	// Execute OnStart hooks (sequential, fail-fast)
	ctx := context.Background()
	if err := a.executeStartHooks(ctx); err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}

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

	return a.runServer(server, server.ListenAndServe, "HTTP")
}

// RunTLS starts the HTTPS server with graceful shutdown.
// This automatically freezes the router before starting, making routes immutable.
func (a *App) RunTLS(addr, certFile, keyFile string) error {
	// Execute OnStart hooks (sequential, fail-fast)
	ctx := context.Background()
	if err := a.executeStartHooks(ctx); err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}

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

	return a.runServer(server, func() error {
		return server.ListenAndServeTLS(certFile, keyFile)
	}, "HTTPS")
}

// RunMTLS starts an HTTPS server with mutual TLS (mTLS) authentication.
// This requires both client and server certificates for bidirectional authentication.
// This automatically freezes the router before starting, making routes immutable.
//
// The server will:
//   - Require client certificates (ClientAuth: RequireAndVerifyClientCert)
//   - Validate client certificates against ClientCAs
//   - Optionally authorize clients using the WithAuthorize callback
//   - Support SNI via WithSNI callback
//   - Support hot-reload via WithConfigForClient callback
//
// Example:
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
//	err = app.RunMTLS(":8443", serverCert,
//	    app.WithClientCAs(caCertPool),
//	    app.WithMinVersion(tls.VersionTLS13),
//	    app.WithAuthorize(func(cert *x509.Certificate) (string, bool) {
//	        // Extract principal from certificate
//	        return cert.Subject.CommonName, cert.Subject.CommonName != ""
//	    }),
//	)
func (a *App) RunMTLS(addr string, serverCert tls.Certificate, opts ...MTLSOption) error {
	// Create mTLS configuration from options
	cfg := newMTLSConfig(serverCert, opts...)

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return fmt.Errorf("invalid mTLS configuration: %w", err)
	}

	// Execute OnStart hooks (sequential, fail-fast)
	ctx := context.Background()
	if err := a.executeStartHooks(ctx); err != nil {
		return fmt.Errorf("startup failed: %w", err)
	}

	// Freeze router before starting (point of no return)
	a.router.Freeze()

	// Build TLS configuration
	tlsConfig := cfg.buildTLSConfig()

	// Create listener
	listener, err := net.Listen("tcp", addr)
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

	// Wrap ConnState callback to extract peer principal from client certificate
	originalConnState := server.ConnState
	server.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateActive {
			if tlsConn, ok := conn.(*tls.Conn); ok {
				// Extract peer certificate
				connState := tlsConn.ConnectionState()
				if len(connState.PeerCertificates) > 0 {
					peerCert := connState.PeerCertificates[0]

					// Authorize if callback provided
					if cfg.authorize != nil {
						_, allowed := cfg.authorize(peerCert)
						if !allowed {
							// Close connection if not authorized
							conn.Close()
							return
						}
						// Principal can be extracted in handlers via request context if needed
					}
				}
			}
		}

		// Call original ConnState if set
		if originalConnState != nil {
			originalConnState(conn, state)
		}
	}

	// Use runServer helper with custom start function for TLS listener
	return a.runServer(server, func() error {
		return server.Serve(tlsListener)
	}, "mTLS")
}
