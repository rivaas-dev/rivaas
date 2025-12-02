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

import (
	"io"
	"log/slog"
)

// WithHandlerType sets the logging handler type.
func WithHandlerType(t HandlerType) Option {
	return func(l *Logger) { l.handlerType = t }
}

// WithJSONHandler uses JSON structured logging (default).
func WithJSONHandler() Option {
	return WithHandlerType(JSONHandler)
}

// WithTextHandler uses text key=value logging.
func WithTextHandler() Option {
	return WithHandlerType(TextHandler)
}

// WithConsoleHandler uses human-readable console logging.
func WithConsoleHandler() Option {
	return WithHandlerType(ConsoleHandler)
}

// WithOutput sets the output writer.
func WithOutput(w io.Writer) Option {
	return func(l *Logger) { l.output = w }
}

// WithLevel sets the minimum log level.
func WithLevel(level Level) Option {
	return func(l *Logger) { l.level = level }
}

// WithDebugLevel enables debug logging.
func WithDebugLevel() Option {
	return WithLevel(LevelDebug)
}

// WithServiceName sets the service name.
// When set, the service name is automatically added to all log entries.
func WithServiceName(name string) Option {
	return func(l *Logger) {
		l.serviceName = name
	}
}

// WithServiceVersion sets the service version.
// When set, the version is automatically added to all log entries.
func WithServiceVersion(version string) Option {
	return func(l *Logger) {
		l.serviceVersion = version
	}
}

// WithEnvironment sets the environment.
// When set, the environment is automatically added to all log entries.
func WithEnvironment(env string) Option {
	return func(l *Logger) {
		l.environment = env
	}
}

// WithSource enables source code location in logs.
func WithSource(enabled bool) Option {
	return func(l *Logger) { l.addSource = enabled }
}

// WithDebugMode enables verbose debugging information.
func WithDebugMode(enabled bool) Option {
	return func(l *Logger) {
		l.debugMode = enabled
		if enabled {
			// Auto-enable source, debug level for diagnostics
			l.addSource = true
			l.level = LevelDebug
		}
	}
}

// WithReplaceAttr sets a custom attribute replacer function.
// The function receives groups and an [slog.Attr], and returns a modified attribute.
// Return an empty [slog.Attr] to drop the attribute from output.
func WithReplaceAttr(fn func(groups []string, a slog.Attr) slog.Attr) Option {
	return func(l *Logger) { l.replaceAttr = fn }
}

// WithCustomLogger uses a custom [slog.Logger] instead of creating one.
// When using a custom logger, [Logger.SetLevel] is not supported.
func WithCustomLogger(customLogger *slog.Logger) Option {
	return func(l *Logger) {
		l.customLogger = customLogger
		l.useCustom = true
	}
}

// WithGlobalLogger registers this logger as the global slog default logger.
// By default, loggers are not registered globally to allow multiple logger
// instances to coexist in the same process.
//
// Example:
//
//	logger := logging.MustNew(
//	    logging.WithJSONHandler(),
//	    logging.WithGlobalLogger(), // Register as global default
//	)
func WithGlobalLogger() Option {
	return func(l *Logger) {
		l.registerGlobal = true
	}
}

// WithSampling enables log sampling to reduce volume in high-traffic scenarios.
// See [SamplingConfig] for configuration options.
func WithSampling(cfg SamplingConfig) Option {
	return func(l *Logger) {
		l.samplingConfig = &cfg
	}
}
