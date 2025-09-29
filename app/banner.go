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
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/common-nighthawk/go-figure"
	"golang.org/x/term"
)

// getColorWriter returns a colorprofile.Writer configured for the app's environment.
// In production mode, ANSI colors are stripped. In development, colors are
// automatically downsampled based on terminal capabilities.
func (a *App) getColorWriter(w io.Writer) *colorprofile.Writer {
	cpw := colorprofile.NewWriter(w, os.Environ())
	// In production, explicitly strip all ANSI sequences
	if a.config.environment == EnvironmentProduction {
		cpw.Profile = colorprofile.NoTTY
	}
	return cpw
}

// printStartupBanner prints an ASCII art startup banner with service information.
// printStartupBanner displays dynamically generated ASCII art of the service name along with version, environment, address, and routes.
func (a *App) printStartupBanner(addr, protocol string) {
	w := a.getColorWriter(os.Stdout)

	// Generate ASCII art from service name using go-figure
	// Using "standard" font as default (can be customized), strict mode disabled for safety
	myFigure := figure.NewFigure(a.config.serviceName, "", false)
	asciiLines := myFigure.Slicify()

	// Apply gradient color effect based on environment
	var gradientColors []string
	if a.config.environment == EnvironmentDevelopment {
		gradientColors = []string{"12", "14", "10", "11"} // Blue, Cyan, Green, Yellow
	} else {
		gradientColors = []string{"10", "11"} // Green, Yellow
	}

	// Create styled ASCII art with gradient effect
	var styledArt strings.Builder
	for _, line := range asciiLines {
		if strings.TrimSpace(line) == "" {
			_, _ = styledArt.WriteString("\n") //nolint:errcheck // strings.Builder.WriteString rarely fails
			continue
		}
		for i, char := range line {
			colorIndex := i % len(gradientColors)
			color := gradientColors[colorIndex]
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(color)).
				Bold(true)
			_, _ = styledArt.WriteString(style.Render(string(char))) //nolint:errcheck // strings.Builder.WriteString rarely fails
		}
		_, _ = styledArt.WriteString("\n") //nolint:errcheck // strings.Builder.WriteString rarely fails
	}

	// Create a compact info box with vertical layout
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(12).
		Align(lipgloss.Right)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Bold(true)

	// Normalize address display: ":8080" -> "0.0.0.0:8080"
	displayAddr := addr
	if strings.HasPrefix(addr, ":") {
		displayAddr = "0.0.0.0" + addr
	}

	// Prepend scheme based on protocol
	scheme := "http://"
	if protocol == "HTTPS" || protocol == "mTLS" {
		scheme = "https://"
	}
	displayAddr = scheme + displayAddr

	versionLabel := labelStyle.Render("Version:")
	versionValue := valueStyle.Foreground(lipgloss.Color("14")).Render(a.config.serviceVersion)
	envLabel := labelStyle.Render("Environment:")
	envValue := valueStyle.Foreground(lipgloss.Color("11")).Render(a.config.environment)
	addrLabel := labelStyle.Render("Address:")
	addrValue := valueStyle.Foreground(lipgloss.Color("10")).Render(displayAddr)

	// Build info box content
	infoLines := []string{
		versionLabel + "  " + versionValue,
		envLabel + "  " + envValue,
		addrLabel + "  " + addrValue,
	}

	// Always show observability info with status
	metricsLabel := labelStyle.Render("Metrics:")
	var metricsValue string
	if a.metrics != nil {
		metricsAddr := a.metrics.GetServerAddress()
		// Normalize metrics address: ":9090" -> "0.0.0.0:9090"
		if strings.HasPrefix(metricsAddr, ":") {
			metricsAddr = "0.0.0.0" + metricsAddr
		}
		// Prepend scheme (metrics server is always HTTP) and append path
		metricsPath := a.metrics.Path()
		if metricsPath == "" {
			metricsPath = "/metrics" // Default path
		}
		metricsAddr = "http://" + metricsAddr + metricsPath
		metricsValue = valueStyle.Foreground(lipgloss.Color("13")).Render(metricsAddr)
	} else {
		metricsValue = valueStyle.Foreground(lipgloss.Color("240")).Render("Disabled")
	}
	infoLines = append(infoLines, metricsLabel+"  "+metricsValue)

	tracingLabel := labelStyle.Render("Tracing:")
	var tracingValue string
	if a.tracing != nil {
		tracingValue = valueStyle.Foreground(lipgloss.Color("12")).Render("Enabled")
	} else {
		tracingValue = valueStyle.Foreground(lipgloss.Color("240")).Render("Disabled")
	}
	infoLines = append(infoLines, tracingLabel+"  "+tracingValue)

	// Create compact info box
	infoContent := strings.Join(infoLines, "\n")

	_, _ = fmt.Fprintln(w)                   //nolint:errcheck // Display output, errors are non-critical
	_, _ = fmt.Fprint(w, styledArt.String()) //nolint:errcheck // Display output, errors are non-critical
	_, _ = fmt.Fprintln(w)                   //nolint:errcheck // Display output, errors are non-critical
	_, _ = fmt.Fprint(w, infoContent)        //nolint:errcheck // Display output, errors are non-critical
	_, _ = fmt.Fprintln(w)                   //nolint:errcheck // Display output, errors are non-critical

	// Add routes section (only in development mode)
	if a.config.environment == EnvironmentDevelopment {
		routes := a.router.Routes()
		if len(routes) > 0 {
			_, _ = fmt.Fprintln(w) //nolint:errcheck // Display output, errors are non-critical
			a.renderRoutesTable(w, 80)
		}
	}

	_, _ = fmt.Fprintln(w) //nolint:errcheck // Display output, errors are non-critical
}

// renderRoutesTable renders the routes table to the given writer.
// renderRoutesTable is an internal helper method used by both PrintRoutes and the startup banner.
// width specifies the table width (80 for banner, 120 for standalone).
func (a *App) renderRoutesTable(w io.Writer, width int) {
	routes := a.router.Routes()
	if len(routes) == 0 {
		return
	}

	// Define styles for different HTTP methods
	methodStyles := map[string]lipgloss.Style{
		"GET":     lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true), // Green
		"POST":    lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true), // Blue
		"PUT":     lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true), // Yellow
		"DELETE":  lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),  // Red
		"PATCH":   lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true), // Magenta
		"HEAD":    lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true), // Cyan
		"OPTIONS": lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Bold(true),  // Gray
	}

	// Style for version column
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true) // Orange

	// Determine if we should use colors (only in development, Writer checks terminal)
	useColors := a.config.environment == EnvironmentDevelopment

	// Build table rows and calculate content width
	rows := make([][]string, 0, len(routes))
	maxMethodWidth := len("Method")
	maxVersionWidth := len("Version")
	maxPathWidth := len("Path")
	maxHandlerWidth := len("Handler")

	for _, route := range routes {
		method := route.Method
		if useColors {
			if style, ok := methodStyles[method]; ok {
				method = style.Render(method)
			}
		}

		// Format version field (show "-" if empty, style if present)
		version := route.Version
		if version == "" {
			version = "-"
		} else if useColors {
			version = versionStyle.Render(version)
		}

		// Calculate content widths (use original values, not styled ones, for accurate measurement)
		if len(route.Method) > maxMethodWidth {
			maxMethodWidth = len(route.Method)
		}

		versionLen := len(route.Version)
		if versionLen == 0 {
			versionLen = 1 // "-" is 1 char
		}
		if versionLen > maxVersionWidth {
			maxVersionWidth = versionLen
		}

		if len(route.Path) > maxPathWidth {
			maxPathWidth = len(route.Path)
		}

		if len(route.HandlerName) > maxHandlerWidth {
			maxHandlerWidth = len(route.HandlerName)
		}

		rows = append(rows, []string{
			method,
			version,
			route.Path,
			route.HandlerName,
		})
	}

	// Calculate minimum width needed: borders + separators + padding + content
	// Border chars: left (1) + right (1) = 2
	// Separators: 3 vertical bars between 4 columns = 3
	// Padding: 2 chars per column (left + right) * 4 columns = 8
	// Content: sum of max widths for each column
	minWidth := 2 + 3 + 8 + maxMethodWidth + maxVersionWidth + maxPathWidth + maxHandlerWidth

	// Try to get terminal width if available
	// First try to extract file from wrapped writer, then try os.Stdout directly
	terminalWidth := width // Use provided width as fallback

	var file *os.File
	if f, ok := w.(*os.File); ok {
		file = f
	} else {
		// Try os.Stdout as fallback (most common case)
		file = os.Stdout
	}

	if termWidth, _, err := getTerminalSize(file); err == nil && termWidth > 0 {
		terminalWidth = termWidth
	}

	// Determine final table width:
	// - Use calculated minimum if it's larger than provided width
	// - But don't exceed terminal width
	// - Ensure minimum of 60 characters
	tableWidth := max(minWidth, width)
	if terminalWidth > 0 {
		tableWidth = min(tableWidth, terminalWidth)
	}
	tableWidth = max(60, tableWidth)

	// Create table with lipgloss/table
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(func() lipgloss.Style {
			if useColors {
				return lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray border
			}
			return lipgloss.NewStyle() // No color for border
		}()).
		StyleFunc(func(row, _ int) lipgloss.Style {
			style := lipgloss.NewStyle().
				Align(lipgloss.Left).
				Padding(0, 1)

			// Header row styling
			if row == 0 && useColors {
				style = style.
					Bold(true).
					Foreground(lipgloss.Color("230")) // Light yellow/white
			}

			return style
		}).
		Headers("Method", "Version", "Path", "Handler").
		Rows(rows...).
		Width(tableWidth)

	// Write to writer
	_, _ = fmt.Fprintln(w, t.Render()) //nolint:errcheck // Display output, errors are non-critical
}

// getTerminalSize attempts to get the terminal size using the golang.org/x/term package.
//
// getTerminalSize uses a cross-platform API that works on Unix-like systems (Linux, macOS, BSD)
// and Windows. The package handles platform-specific syscalls internally.
//
// Platform behavior:
//   - Unix/Linux/macOS: Uses TIOCGWINSZ ioctl
//   - Windows: Uses GetConsoleScreenBufferInfo
//   - Non-TTY (pipes, redirects): Returns error (no terminal attached)
//
// getTerminalSize is suitable for synchronous use during startup banner rendering.
// No caching needed as it's called once per startup.
//
// getTerminalSize returns width, height in character cells, or error if terminal size unavailable.
func getTerminalSize(file *os.File) (int, int, error) {
	if file == nil {
		return 0, 0, fmt.Errorf("file is nil")
	}

	width, height, err := term.GetSize(int(file.Fd()))
	if err != nil {
		return 0, 0, fmt.Errorf("unable to get terminal size: %w", err)
	}
	return width, height, nil
}

// PrintRoutes prints all registered routes to stdout in a formatted table.
// PrintRoutes is useful for development and debugging to see all available routes.
//
// PrintRoutes uses lipgloss/table for terminal output with color-coded HTTP methods
// and proper table formatting. A colorprofile.Writer automatically downsamples
// ANSI colors to match the terminal's capabilities (TrueColor → ANSI256 → ANSI).
// If output is not a TTY, ANSI sequences are stripped entirely. This respects
// the NO_COLOR environment variable and handles all terminal capability detection
// automatically.
//
// Colors are only enabled in development mode.
//
// Example output:
//
//	┌────────┬─────────┬──────────────────┬──────────────────┐
//	│ Method │ Version │ Path             │ Handler          │
//	├────────┼─────────┼──────────────────┼──────────────────┤
//	│ GET    │ -       │ /                │ handler          │
//	│ GET    │ v1      │ /users/:id       │ handler          │
//	│ POST   │ -       │ /users           │ handler          │
//	└────────┴─────────┴──────────────────┴──────────────────┘
func (a *App) PrintRoutes() {
	routes := a.router.Routes()
	if len(routes) == 0 {
		_, _ = fmt.Println("No routes registered") //nolint:errcheck // Display output, errors are non-critical
		return
	}

	// Create a writer that automatically downsamples colors based on terminal capabilities
	// Uses helper method that handles production mode (strips ANSI) and development mode (auto-detects)
	w := a.getColorWriter(os.Stdout)

	// Use internal helper with wider table for standalone use
	a.renderRoutesTable(w, 120)
}
