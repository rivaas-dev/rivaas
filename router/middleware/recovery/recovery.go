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

package recovery

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/term"

	"rivaas.dev/router"
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// stackFrame represents a parsed stack trace frame.
type stackFrame struct {
	function string
	file     string
	line     string
	isStdLib bool
}

// Option defines functional options for recovery middleware configuration.
type Option func(*config)

// config holds the configuration for the recovery middleware.
type config struct {
	logger      *slog.Logger
	handler     func(c *router.Context, err any)
	stackTrace  bool
	stackSize   int
	prettyStack *bool // nil = auto-detect, true/false = explicit
}

// defaultConfig returns the default configuration for recovery middleware.
func defaultConfig() *config {
	return &config{
		logger:      slog.Default(), // Logging enabled by default
		handler:     defaultHandler,
		stackTrace:  true,
		stackSize:   4 << 10, // 4KB
		prettyStack: nil,     // Auto-detect based on TTY
	}
}

// defaultHandler sends a 500 Internal Server Error response.
func defaultHandler(c *router.Context, _ any) {
	c.JSON(http.StatusInternalServerError, map[string]any{
		"error": "Internal server error",
		"code":  "INTERNAL_ERROR",
	})
}

// New returns a middleware that recovers from panics in request handlers.
// By default, panics are logged using slog.Default() and return 500.
//
// This middleware should typically be registered first (or early) in the middleware chain
// to catch panics from all subsequent handlers.
//
// Basic usage:
//
//	r := router.MustNew()
//	r.Use(recovery.New())
//
// Disable logging (useful for tests):
//
//	r.Use(recovery.New(recovery.WithoutLogging()))
//
// With custom logger:
//
//	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
//	r.Use(recovery.New(recovery.WithLogger(logger)))
//
// Custom handler:
//
//	r.Use(recovery.New(
//	    recovery.WithHandler(func(c *router.Context, err any) {
//	        c.JSON(http.StatusInternalServerError, map[string]any{
//	            "error":      "Internal server error",
//	            "request_id": c.Header("X-Request-ID"),
//	        })
//	    }),
//	))
func New(opts ...Option) router.HandlerFunc {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return func(c *router.Context) {
		defer func() {
			if err := recover(); err != nil {
				handlePanic(c, cfg, err)
			}
		}()

		c.Next()
	}
}

// handlePanic processes a recovered panic.
func handlePanic(c *router.Context, cfg *config, err any) {
	// Record to OpenTelemetry span if available
	recordPanicToSpan(c, err)

	// Log if logger is configured
	if cfg.logger != nil {
		// Log main error with structured fields
		cfg.logger.Error("panic recovered",
			"error", fmt.Sprintf("%v", err),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
		)

		// Print formatted stack trace
		if cfg.stackTrace {
			stack := captureStack(cfg.stackSize)
			printStackTrace(cfg, stack)
		}
	}

	// Send error response
	if cfg.handler != nil {
		cfg.handler(c, err)
	}
}

// recordPanicToSpan records panic information to the active span if available.
func recordPanicToSpan(c *router.Context, err any) {
	span := c.Span()
	if span == nil || !span.SpanContext().IsValid() {
		return
	}

	span.SetStatus(codes.Error, "panic recovered")
	span.SetAttributes(
		attribute.Bool("exception.escaped", true), // KEY: only set for panics
		attribute.String("exception.type", fmt.Sprintf("%T", err)),
		attribute.String("exception.message", fmt.Sprintf("%v", err)),
	)

	// Record as error event if it's an actual error type
	if actualErr, ok := err.(error); ok {
		span.RecordError(actualErr)
	}
}

// shouldPrettyPrint determines if stack traces should be pretty-printed.
// Returns true if explicitly enabled, or auto-detects based on TTY.
func shouldPrettyPrint(cfg *config) bool {
	if cfg.prettyStack != nil {
		return *cfg.prettyStack
	}
	// Auto-detect: pretty for terminal, compact for files/pipes
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// printStackTrace prints a formatted stack trace.
// Pretty-prints to stderr if in a terminal, otherwise logs compactly.
func printStackTrace(cfg *config, stack []byte) {
	if shouldPrettyPrint(cfg) {
		printColorizedStack(stack)
	} else {
		// Compact output for log aggregators
		cfg.logger.Error("stack trace", "frames", string(stack))
	}
}

// printColorizedStack prints a beautifully formatted, colorized stack trace.
func printColorizedStack(stack []byte) {
	frames := parseStackFrames(string(stack))
	if len(frames) == 0 {
		return
	}

	// Header
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %s%s╭─ Stack Trace%s\n", colorBold, colorRed, colorReset)
	fmt.Fprintf(os.Stderr, "  %s│%s\n", colorRed, colorReset)

	// Print frames
	for i, frame := range frames {
		isLast := i == len(frames)-1
		connector := "├"
		if isLast {
			connector = "╰"
		}

		// Dim stdlib/runtime frames
		funcColor := colorCyan
		fileColor := colorGray
		lineColor := colorYellow
		if frame.isStdLib {
			funcColor = colorDim
			fileColor = colorDim
			lineColor = colorDim
		}

		// Function name
		fmt.Fprintf(os.Stderr, "  %s%s──%s %s%s%s\n",
			colorRed, connector, colorReset,
			funcColor, frame.function, colorReset)

		// File location (indented)
		indent := "│  "
		if isLast {
			indent = "   "
		}
		fmt.Fprintf(os.Stderr, "  %s%s%s    %s%s%s:%s%s%s\n",
			colorRed, indent, colorReset,
			fileColor, frame.file, colorReset,
			lineColor, frame.line, colorReset)
	}

	fmt.Fprintln(os.Stderr)
}

// parseStackFrames parses raw stack trace bytes into structured frames.
func parseStackFrames(stack string) []stackFrame {
	lines := strings.Split(stack, "\n")
	var frames []stackFrame

	// Regex to extract line number from file path (e.g., "/path/file.go:123 +0x5e")
	lineRegex := regexp.MustCompile(`^(.+):(\d+)`)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines and goroutine header
		if line == "" || strings.HasPrefix(line, "goroutine") {
			continue
		}

		// File lines start with "/" or "\t" (after trimming, just "/")
		// Function lines are everything else except "created by" which we handle specially
		isFileLine := strings.HasPrefix(line, "/")
		isCreatedBy := strings.HasPrefix(line, "created by")

		if !isFileLine && !isCreatedBy {
			funcName := cleanFunctionName(line)

			// Next line should be the file location
			if i+1 < len(lines) {
				fileLine := strings.TrimSpace(lines[i+1])
				file, lineNum := parseFileLine(fileLine, lineRegex)

				frames = append(frames, stackFrame{
					function: funcName,
					file:     file,
					line:     lineNum,
					isStdLib: isStandardLibrary(file),
				})
				i++ // Skip the file line
			}
		}
	}

	// Skip internal recovery frames (debug.Stack, captureStack, handlePanic)
	skipFrames := 0
	for i, f := range frames {
		if strings.Contains(f.function, "captureStack") ||
			strings.Contains(f.function, "debug.Stack") ||
			strings.Contains(f.function, "handlePanic") {
			skipFrames = i + 1
		}
	}

	if skipFrames < len(frames) {
		frames = frames[skipFrames:]
	}

	return frames
}

// cleanFunctionName removes argument details from function signature.
// Handles method receivers like (*Type).Method properly.
// Also simplifies anonymous function names like ".func1" to "(λ)".
//
// Examples:
//   - "pkg.Func(0xc000...)" → "pkg.Func()"
//   - "pkg.(*Type).Method(...)" → "pkg.(*Type).Method()"
//   - "pkg.Type.Method(args)" → "pkg.Type.Method()"
//   - "pkg.Handler.func1" → "pkg.Handler(λ)"
func cleanFunctionName(fn string) string {
	// Handle "created by" prefix
	fn = strings.TrimPrefix(fn, "created by ")

	// Find argument patterns at the end:
	// - "(0x..." - runtime arguments with addresses
	// - "(...)" - inlined/optimized functions
	// - "({..." - struct arguments
	// - "()" - no arguments
	argPatterns := []string{"(0x", "(...)", "({", "()"}
	for _, pattern := range argPatterns {
		if idx := strings.LastIndex(fn, pattern); idx > 0 {
			fn = fn[:idx]
			break
		}
	}

	// Fallback: if ends with ), find the matching ( that's after an identifier
	if strings.HasSuffix(fn, ")") {
		depth := 0
		for i := len(fn) - 1; i >= 0; i-- {
			switch fn[i] {
			case ')':
				depth++
			case '(':
				depth--
				if depth == 0 {
					// Check if preceded by "." (receiver type like ".(*Type)")
					// or preceded by an identifier character (method/func args)
					if i > 0 {
						prev := fn[i-1]
						// If preceded by ".", this is a receiver type - don't cut here
						if prev == '.' {
							continue
						}
						// If preceded by identifier char, this is function args - cut here
						if isIdentChar(prev) {
							fn = fn[:i]
							break
						}
					}
				}
			}
		}
	}

	// Simplify anonymous function names: ".func1", ".func1.1" → "(λ)"
	// Go names anonymous functions as FuncName.func1, FuncName.func1.1, etc.
	fn = simplifyAnonFunc(fn)

	// Add () to function names that don't already have parentheses
	// This makes output consistent: Method() vs Handler(λ)
	if len(fn) > 0 && !strings.HasSuffix(fn, ")") {
		fn += "()"
	}

	return fn
}

// simplifyAnonFunc converts Go's anonymous function names to a readable format.
// ".func1", ".func1.1", ".func2" etc. become "(λ)"
func simplifyAnonFunc(fn string) string {
	idx := strings.Index(fn, ".func")
	if idx <= 0 {
		return fn
	}

	// Check if what follows ".func" is just digits and dots (e.g., "1", "1.1")
	suffix := fn[idx+5:] // after ".func"
	if len(suffix) == 0 {
		return fn
	}

	for _, c := range suffix {
		if c != '.' && (c < '0' || c > '9') {
			return fn // Not an anonymous function pattern
		}
	}

	// It's an anonymous function - replace with (λ)
	return fn[:idx] + "(λ)"
}

// isIdentChar returns true if c is a valid Go identifier character.
func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_'
}

// parseFileLine extracts file path and line number.
func parseFileLine(line string, re *regexp.Regexp) (file, lineNum string) {
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 3 {
		return shortenPath(matches[1]), matches[2]
	}
	return line, ""
}

// shortenPath shortens common path prefixes for readability.
func shortenPath(path string) string {
	// Shorten home directory
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	// Shorten Go module cache
	if idx := strings.Index(path, "/pkg/mod/"); idx > 0 {
		path = path[idx+9:] // Remove everything before /pkg/mod/
	}
	return path
}

// isStandardLibrary checks if a file path is from the Go standard library or runtime.
func isStandardLibrary(path string) bool {
	return strings.Contains(path, "/go/src/") ||
		strings.Contains(path, "go-1.") ||
		strings.HasPrefix(path, "runtime/") ||
		strings.HasPrefix(path, "net/http")
}

// captureStack captures the current goroutine's stack trace.
func captureStack(maxSize int) []byte {
	stack := debug.Stack()
	if len(stack) > maxSize {
		return stack[:maxSize]
	}
	return stack
}
