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

	"rivaas.dev/router/route"
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
			_, _ = styledArt.WriteString("\n")
			continue
		}
		for i, char := range line {
			colorIndex := i % len(gradientColors)
			color := gradientColors[colorIndex]
			style := lipgloss.NewStyle().
				Foreground(lipgloss.Color(color)).
				Bold(true)
			_, _ = styledArt.WriteString(style.Render(string(char)))
		}
		_, _ = styledArt.WriteString("\n")
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
	//nolint:errcheck // Best-effort banner display; write errors don't affect functionality
	_, _ = fmt.Fprintln(w)
	//nolint:errcheck // Best-effort banner display
	_, _ = fmt.Fprint(w, styledArt.String())
	//nolint:errcheck // Best-effort banner display
	_, _ = fmt.Fprintln(w)
	//nolint:errcheck // Best-effort banner display
	_, _ = fmt.Fprint(w, output.String())

	// Add routes section (only in development mode)
	if a.config.environment == EnvironmentDevelopment {
		routes := a.router.Routes()
		if len(routes) > 0 {
			//nolint:errcheck // Best-effort banner display
			_, _ = fmt.Fprintln(w)
			a.renderRoutesTable(w)
		}
	}

	//nolint:errcheck // Best-effort banner display
	_, _ = fmt.Fprintln(w)
}

// renderRoutesTable renders the routes table to the given writer.
// renderRoutesTable is an internal helper method used by both PrintRoutes and the startup banner.
// Columns are dynamically sized based on content.
func (a *App) renderRoutesTable(w io.Writer) {
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

	// Separate builtin and user routes
	var builtinRoutes, userRoutes []route.Info
	for _, r := range routes {
		if strings.HasPrefix(r.HandlerName, "[builtin]") {
			builtinRoutes = append(builtinRoutes, r)
		} else {
			userRoutes = append(userRoutes, r)
		}
	}

	// Build table rows: builtin routes first, then user routes
	rows := make([][]string, 0, len(routes))
	dimStyle := lipgloss.NewStyle().Faint(true) // Dim style for builtin routes

	// Add builtin routes first
	for _, route := range builtinRoutes {
		method := route.Method
		if useColors {
			if style, ok := methodStyles[method]; ok {
				method = style.Render(method)
			}
		}

		version := route.Version
		if version == "" {
			version = "-"
		} else if useColors {
			version = versionStyle.Render(version)
		}

		handlerName := route.HandlerName
		if useColors {
			handlerName = dimStyle.Render(handlerName)
		}

		rows = append(rows, []string{
			method,
			version,
			route.Path,
			handlerName,
		})
	}

	// Add user routes
	for _, route := range userRoutes {
		method := route.Method
		if useColors {
			if style, ok := methodStyles[method]; ok {
				method = style.Render(method)
			}
		}

		version := route.Version
		if version == "" {
			version = "-"
		} else if useColors {
			version = versionStyle.Render(version)
		}

		rows = append(rows, []string{
			method,
			version,
			route.Path,
			route.HandlerName,
		})
	}

	// Create table with lipgloss/table
	// Let columns auto-size based on content (no fixed width)
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(func() lipgloss.Style {
			if useColors {
				return lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Gray border
			}

			return lipgloss.NewStyle() // No color for border
		}()).
		StyleFunc(func(row, _ int) lipgloss.Style {
			// Note: In lipgloss/table, row indexing starts at 0 for the first DATA row
			// Headers are handled separately by the Headers() method
			style := lipgloss.NewStyle().
				Align(lipgloss.Left).
				Padding(0, 1)

			return style
		}).
		Headers("Method", "Version", "Path", "Handler").
		Rows(rows...)

	//nolint:errcheck // Best-effort table display
	_, _ = fmt.Fprintln(w, t.Render())
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
		_, _ = fmt.Println("No routes registered")
		return
	}

	// Create a writer that automatically downsamples colors based on terminal capabilities
	// Uses helper method that handles production mode (strips ANSI) and development mode (auto-detects)
	w := a.getColorWriter(os.Stdout)

	// Use internal helper with wider table for standalone use
	a.renderRoutesTable(w)
}
