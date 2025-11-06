// Package main demonstrates error logging with stack traces.
package main

import (
	"errors"

	"rivaas.dev/logging"
)

func main() {
	logger := logging.MustNew(logging.WithConsoleHandler())

	err := errors.New("database connection failed")
	logger.Error("without stack", "error", err)

	logger.ErrorWithStack("with stack", err, true)
}
