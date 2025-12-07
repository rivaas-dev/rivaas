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
	"net/http"
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

// printStartupBanner prints the startup banner to stdout.
// It is called by [App.Run], [App.RunTLS], and [App.RunMTLS].
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

	// Define styles for categorized banner
	categoryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Width(14).
		PaddingLeft(2).
		Align(lipgloss.Left)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Bold(true)

	disabledStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	providerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("243")) // Dimmed gray for provider brackets

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

	// Build categorized sections
	var output strings.Builder

	// === Service Section ===
	_, _ = output.WriteString(categoryStyle.Render("Service") + "\n")
	_, _ = output.WriteString(labelStyle.Render("Version:") + "  " + valueStyle.Foreground(lipgloss.Color("14")).Render(a.config.serviceVersion) + "\n")
	_, _ = output.WriteString(labelStyle.Render("Environment:") + "  " + valueStyle.Foreground(lipgloss.Color("11")).Render(a.config.environment) + "\n")
	_, _ = output.WriteString(labelStyle.Render("Address:") + "  " + valueStyle.Foreground(lipgloss.Color("10")).Render(displayAddr) + "\n")

	// === Observability Section ===
	_, _ = output.WriteString("\n" + categoryStyle.Render("Observability") + "\n")

	// Metrics
	var metricsLine string
	if a.metrics != nil {
		metricsAddr := a.metrics.ServerAddress()
		if strings.HasPrefix(metricsAddr, ":") {
			metricsAddr = "0.0.0.0" + metricsAddr
		}
		metricsPath := a.metrics.Path()
		if metricsPath == "" {
			metricsPath = "/metrics"
		}
		metricsAddr = "http://" + metricsAddr + metricsPath
		metricsLine = labelStyle.Render("Metrics:") + "  " +
			valueStyle.Foreground(lipgloss.Color("13")).Render(metricsAddr) + "  " +
			providerStyle.Render(fmt.Sprintf("[%s]", a.metrics.Provider()))
	} else {
		metricsLine = labelStyle.Render("Metrics:") + "  " + disabledStyle.Render("Disabled")
	}
	_, _ = output.WriteString(metricsLine + "\n")

	// Tracing
	var tracingLine string
	if a.tracing != nil {
		tracingLine = labelStyle.Render("Tracing:") + "  " +
			valueStyle.Foreground(lipgloss.Color("12")).Render("Enabled") + "  " +
			providerStyle.Render(fmt.Sprintf("[%s]", a.tracing.GetProvider()))
	} else {
		tracingLine = labelStyle.Render("Tracing:") + "  " + disabledStyle.Render("Disabled")
	}
	_, _ = output.WriteString(tracingLine + "\n")

	// === Documentation Section ===
	if a.openapi != nil {
		_, _ = output.WriteString("\n" + categoryStyle.Render("Documentation") + "\n")

		// Always show API Docs (Swagger UI) if enabled
		if a.openapi.ServeUI() {
			docsAddr := displayAddr + a.openapi.UIPath()
			_, _ = output.WriteString(labelStyle.Render("API Docs:") + "  " + valueStyle.Foreground(lipgloss.Color("14")).Render(docsAddr) + "\n")
		}

		// Always show OpenAPI spec endpoint
		specAddr := displayAddr + a.openapi.SpecPath()
		_, _ = output.WriteString(labelStyle.Render("OpenAPI:") + "  " + valueStyle.Foreground(lipgloss.Color("14")).Render(specAddr) + "\n")
	}

	// Print the banner
	_, _ = fmt.Fprintln(w)                   //nolint:errcheck // Display output, errors are non-critical
	_, _ = fmt.Fprint(w, styledArt.String()) //nolint:errcheck // Display output, errors are non-critical
	_, _ = fmt.Fprintln(w)                   //nolint:errcheck // Display output, errors are non-critical
	_, _ = fmt.Fprint(w, output.String())    //nolint:errcheck // Display output, errors are non-critical

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
		http.MethodGet:     lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true), // Green
		http.MethodPost:    lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true), // Blue
		http.MethodPut:     lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true), // Yellow
		http.MethodDelete:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),  // Red
		http.MethodPatch:   lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true), // Magenta
		http.MethodHead:    lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true), // Cyan
		http.MethodOptions: lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Bold(true),  // Gray
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
		maxMethodWidth = max(maxMethodWidth, len(route.Method))

		versionLen := len(route.Version)
		if versionLen == 0 {
			versionLen = 1 // "-" is 1 char
		}
		maxVersionWidth = max(maxVersionWidth, versionLen)

		maxPathWidth = max(maxPathWidth, len(route.Path))
		maxHandlerWidth = max(maxHandlerWidth, len(route.HandlerName))

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
	// Only check terminal size if writer is an *os.File (avoids race with tests)
	terminalWidth := width // Use provided width as fallback

	if file, ok := w.(*os.File); ok {
		if termWidth, _, err := getTerminalSize(file); err == nil && termWidth > 0 {
			terminalWidth = termWidth
		}
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
// It is useful for development and debugging to see all available routes.
//
// It uses lipgloss/table for terminal output with color-coded HTTP methods
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
