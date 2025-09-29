// Package main demonstrates standalone request logging functionality.
package main

import (
	"net/http"

	"rivaas.dev/logging"
)

func main() {
	logger := logging.MustNew(logging.WithConsoleHandler())

	req, _ := http.NewRequest("GET", "https://example.com/api/items?id=42", nil)
	req.Header.Set("User-Agent", "example-client/1.0")

	// Standalone request logging (no router/tracing/metrics)
	logger.LogRequest(req, "status", 200, "duration_ms", 12)
}
