// Package main demonstrates context-only field logging.
package main

import (
	"context"

	"rivaas.dev/logging"
)

func main() {
	logger := logging.MustNew(logging.WithConsoleHandler())

	// Carry IDs in context (no tracing). Extract and attach as structured fields.
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxKey("request_id"), "req-123")
	ctx = context.WithValue(ctx, ctxKey("user_id"), "u-42")

	requestID, _ := ctx.Value(ctxKey("request_id")).(string)
	userID, _ := ctx.Value(ctxKey("user_id")).(string)

	reqLogger := logger.Logger().With(
		"request.id", requestID,
		"user.id", userID,
	)

	reqLogger.Info("handling request")
	reqLogger.Info("request completed")
}

type ctxKey string
