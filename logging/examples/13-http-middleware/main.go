// Package main demonstrates how to use the logging middleware with HTTP handlers.
package main

import (
	"net/http"

	"rivaas.dev/logging"
)

func main() {
	// Create logger with JSON output
	log := logging.MustNew(
		logging.WithJSONHandler(),
		logging.WithServiceName("http-api"),
		logging.WithServiceVersion("v1.0.0"),
		logging.WithEnvironment("production"),
	)

	// Create HTTP middleware
	mw := logging.Middleware(log,
		logging.WithSkipPaths("/health", "/metrics"), // Don't log health checks
		logging.WithLogHeaders(false),                // Don't log headers for privacy
	)

	// Example handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Hello, World!"}`))
	})

	// Wrap handler with logging middleware
	http.Handle("/api/hello", mw(handler))

	// Health check endpoint (not logged due to WithSkipPaths)
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Info("Server starting", "port", 8080)
	http.ListenAndServe(":8080", nil)
}
