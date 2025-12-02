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

package logging

import "errors"

// Error types for better error handling and testing.
//
// Design rationale:
//   - Sentinel errors (package-level vars) enable [errors.Is] checks
//   - Descriptive names make error handling self-documenting
//   - Explicit error types improve testability vs string comparison
//
// Usage pattern:
//
//	if err := logger.SetLevel(level); err != nil {
//	    if errors.Is(err, logging.ErrCannotChangeLevel) {
//	        // Handle immutable logger case
//	    } else {
//	        // Handle other errors
//	    }
//	}
var (
	// ErrNilLogger indicates a nil custom logger was provided to [WithCustomLogger].
	// This is a programmer error and should be caught during initialization.
	ErrNilLogger = errors.New("custom logger is nil")

	// ErrInvalidHandler indicates an unsupported handler type was specified.
	// Valid types: JSONHandler, TextHandler, ConsoleHandler.
	ErrInvalidHandler = errors.New("invalid handler type")

	// ErrLoggerShutdown indicates the logger has been shut down via [Logger.Shutdown].
	// Further log attempts are silently dropped (not an error condition).
	// This error is returned by operations that require an active logger.
	ErrLoggerShutdown = errors.New("logger is shut down")

	// ErrInvalidLevel indicates an invalid log level was provided.
	// Valid levels: LevelDebug, LevelInfo, LevelWarn, LevelError.
	ErrInvalidLevel = errors.New("invalid log level")

	// ErrCannotChangeLevel indicates log level cannot be changed dynamically.
	// Returned by [Logger.SetLevel] when using a custom logger (level controlled externally).
	ErrCannotChangeLevel = errors.New("cannot change level on custom logger")
)
