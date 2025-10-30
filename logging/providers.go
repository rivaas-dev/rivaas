package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[37m"
	colorWhite  = "\033[97m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// Pool for strings.Builder to reduce allocations in console handler
var consoleBuilderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

// consoleHandler implements a human-readable colored console handler.
type consoleHandler struct {
	opts   *slog.HandlerOptions
	output io.Writer
	attrs  []slog.Attr
	groups []string
}

// newConsoleHandler creates a new console handler.
func newConsoleHandler(w io.Writer, opts *slog.HandlerOptions) *consoleHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &consoleHandler{
		opts:   opts,
		output: w,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

// Handle formats and writes a log record.
func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	// Get pooled strings.Builder
	b := consoleBuilderPool.Get().(*strings.Builder)
	b.Reset()
	defer consoleBuilderPool.Put(b)

	// Timestamp
	b.WriteString(colorDim)
	b.WriteString(r.Time.Format("15:04:05.000"))
	b.WriteString(colorReset)
	b.WriteString(" ")

	// Level with color
	b.WriteString(h.levelColor(r.Level))
	b.WriteString(colorBold)
	b.WriteString(fmt.Sprintf("%-5s", r.Level.String()))
	b.WriteString(colorReset)
	b.WriteString(" ")

	// Message
	b.WriteString(colorWhite)
	b.WriteString(r.Message)
	b.WriteString(colorReset)

	// Attributes
	if r.NumAttrs() > 0 || len(h.attrs) > 0 {
		b.WriteString(" ")
		// Pre-existing attributes
		for _, a := range h.attrs {
			h.appendAttr(b, a)
		}
		// Record attributes
		r.Attrs(func(a slog.Attr) bool {
			h.appendAttr(b, a)
			return true
		})
	}

	// Source location
	if h.opts.AddSource && r.PC != 0 {
		if src := recordSource(r.PC); src != "" {
			b.WriteString(" ")
			b.WriteString(colorGray)
			b.WriteString("(" + src + ")")
			b.WriteString(colorReset)
		}
	}

	b.WriteString("\n")

	_, err := h.output.Write([]byte(b.String()))
	return err
}

// WithAttrs returns a new handler with additional attributes.
func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &consoleHandler{
		opts:   h.opts,
		output: h.output,
		attrs:  newAttrs,
		groups: h.groups,
	}
}

// WithGroup returns a new handler with a group name.
func (h *consoleHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups[len(h.groups)] = name
	return &consoleHandler{
		opts:   h.opts,
		output: h.output,
		attrs:  h.attrs,
		groups: newGroups,
	}
}

// levelColor returns the ANSI color code for a log level.
func (h *consoleHandler) levelColor(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return colorRed
	case level >= slog.LevelWarn:
		return colorYellow
	case level >= slog.LevelInfo:
		return colorGreen
	default:
		return colorBlue
	}
}

// appendAttr formats and appends an attribute.
// Type switch is ordered by frequency: string, int, int64, bool, then rest.
func (h *consoleHandler) appendAttr(b *strings.Builder, a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}

	b.WriteString(a.Key)
	b.WriteString("=")

	// Optimized type switch ordered by frequency
	switch v := a.Value.Any().(type) {
	case string:
		b.WriteString(v)
	case int:
		b.WriteString(strconv.Itoa(v))
	case int64:
		b.WriteString(strconv.FormatInt(v, 10))
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case time.Duration:
		b.WriteString(v.String())
	case time.Time:
		b.WriteString(v.Format(time.RFC3339))
	case float64:
		b.WriteString(strconv.FormatFloat(v, 'f', 2, 64))
	case float32:
		b.WriteString(strconv.FormatFloat(float64(v), 'f', 2, 32))
	case int8:
		b.WriteString(strconv.FormatInt(int64(v), 10))
	case int16:
		b.WriteString(strconv.FormatInt(int64(v), 10))
	case int32:
		b.WriteString(strconv.FormatInt(int64(v), 10))
	case uint:
		b.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint8:
		b.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint16:
		b.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint32:
		b.WriteString(strconv.FormatUint(uint64(v), 10))
	case uint64:
		b.WriteString(strconv.FormatUint(v, 10))
	case error:
		b.WriteString(v.Error())
	default:
		// Only use fmt.Sprint as last resort
		b.WriteString(fmt.Sprint(v))
	}

	b.WriteString(" ")
}

// recordSource returns "file:line" for a pc if available.
func recordSource(pc uintptr) string {
	fs := runtime.CallersFrames([]uintptr{pc})
	f, _ := fs.Next()
	if f.File == "" {
		return ""
	}
	// Get just the filename, not the full path
	parts := strings.Split(f.File, "/")
	file := f.File
	if len(parts) > 0 {
		file = parts[len(parts)-1]
	}
	return fmt.Sprintf("%s:%d", file, f.Line)
}
