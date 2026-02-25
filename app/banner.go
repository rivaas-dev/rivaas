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

// bannerWriter provides helper methods for rendering styled banner sections.
// It encapsulates the shared lipgloss styles so callers focus on content, not formatting.
type bannerWriter struct {
	buf           strings.Builder
	categoryStyle lipgloss.Style
	labelStyle    lipgloss.Style
	valueStyle    lipgloss.Style
	disabledStyle lipgloss.Style
	providerStyle lipgloss.Style
}

// newBannerWriter creates a new bannerWriter with default lipgloss styles.
func newBannerWriter() *bannerWriter {
	return &bannerWriter{
		categoryStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Bold(true),
		labelStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Width(14).
			PaddingLeft(2).
			Align(lipgloss.Left),
		valueStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Bold(true),
		disabledStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")),
		providerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")),
	}
}

// category writes a styled section header (e.g., "Service", "Observability").
func (bw *bannerWriter) category(name string) {
	_, _ = bw.buf.WriteString(bw.categoryStyle.Render(name) + "\n")
}

// field writes a single label-value line with the given ANSI color.
func (bw *bannerWriter) field(label, value, color string) {
	_, _ = bw.buf.WriteString(
		bw.labelStyle.Render(label) + "  " +
			bw.valueStyle.Foreground(lipgloss.Color(color)).Render(value) + "\n",
	)
}

// fieldWithProvider writes a label-value line with a dimmed provider suffix.
func (bw *bannerWriter) fieldWithProvider(label, value, color, provider string) {
	_, _ = bw.buf.WriteString(
		bw.labelStyle.Render(label) + "  " +
			bw.valueStyle.Foreground(lipgloss.Color(color)).Render(value) + "  " +
			bw.providerStyle.Render(fmt.Sprintf("[%s]", provider)) + "\n",
	)
}

// disabled writes a label with "Disabled" in dim style.
func (bw *bannerWriter) disabled(label string) {
	_, _ = bw.buf.WriteString(
		bw.labelStyle.Render(label) + "  " + bw.disabledStyle.Render("Disabled") + "\n",
	)
}

// blank writes an empty line (section separator).
func (bw *bannerWriter) blank() {
	_, _ = bw.buf.WriteString("\n")
}

// String returns the accumulated output.
func (bw *bannerWriter) String() string {
	return bw.buf.String()
}

// normalizeAddr prepends "0.0.0.0" to addresses starting with ":" for display clarity.
func normalizeAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "0.0.0.0" + addr
	}
	return addr
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

	// Pre-build gradient styles to avoid repeated allocations
	gradientStyles := make([]lipgloss.Style, 0, len(gradientColors))
	for _, c := range gradientColors {
		gradientStyles = append(gradientStyles, lipgloss.NewStyle().
			Foreground(lipgloss.Color(c)).
			Bold(true))
	}

	// Create styled ASCII art with gradient effect
	var styledArt strings.Builder
	for _, line := range asciiLines {
		if strings.TrimSpace(line) == "" {
			_, _ = styledArt.WriteString("\n")
			continue
		}
		for i, char := range line {
			_, _ = styledArt.WriteString(gradientStyles[i%len(gradientStyles)].Render(string(char)))
		}
		_, _ = styledArt.WriteString("\n")
	}

	displayAddr := normalizeAddr(addr)

	// Prepend scheme based on protocol
	scheme := "http://"
	if protocol == "HTTPS" || protocol == "mTLS" {
		scheme = "https://"
	}
	displayAddr = scheme + displayAddr

	// Build categorized sections using bannerWriter
	bw := newBannerWriter()

	// === Service Section ===
	bw.category("Service")
	bw.field("Version:", a.config.serviceVersion, "14")
	bw.field("Environment:", a.config.environment, "11")
	bw.field("Address:", displayAddr, "10")
	if a.hasReloadHooks() {
		bw.field("Reload:", "SIGHUP (enabled)", "13")
	}

	// === Observability Section ===
	bw.blank()
	bw.category("Observability")

	// Metrics
	if a.metrics != nil {
		metricsAddr := normalizeAddr(a.metrics.ServerAddress())
		metricsPath := a.metrics.Path()
		if metricsPath == "" {
			metricsPath = "/metrics"
		}
		metricsAddr = "http://" + metricsAddr + metricsPath
		bw.fieldWithProvider("Metrics:", metricsAddr, "13", string(a.metrics.Provider()))
	} else {
		bw.disabled("Metrics:")
	}

	// Tracing
	if a.tracing != nil {
		bw.fieldWithProvider("Tracing:", "Enabled", "12", string(a.tracing.GetProvider()))
	} else {
		bw.disabled("Tracing:")
	}

	// === Documentation Section ===
	if a.openapi != nil {
		bw.blank()
		bw.category("Documentation")

		// Always show API Docs (Swagger UI) if enabled
		if a.openapi.ServeUI() {
			docsAddr := displayAddr + a.openapi.UIPath()
			bw.field("API Docs:", docsAddr, "14")
		}

		// Always show OpenAPI spec endpoint
		specAddr := displayAddr + a.openapi.SpecPath()
		bw.field("OpenAPI:", specAddr, "14")
	}

	// Print the banner
	//nolint:errcheck // Best-effort banner display; write errors don't affect functionality
	_, _ = fmt.Fprintln(w)
	//nolint:errcheck // Best-effort banner display
	_, _ = fmt.Fprint(w, styledArt.String())
	//nolint:errcheck // Best-effort banner display
	_, _ = fmt.Fprintln(w)
	//nolint:errcheck // Best-effort banner display
	_, _ = fmt.Fprint(w, bw.String())

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

	// Merge builtin and user routes, preserving order (builtin first)
	allRoutes := append(builtinRoutes, userRoutes...)

	for _, r := range allRoutes {
		isBuiltin := strings.HasPrefix(r.HandlerName, "[builtin]")

		method := r.Method
		if useColors {
			if style, ok := methodStyles[method]; ok {
				method = style.Render(method)
			}
		}

		version := r.Version
		if version == "" {
			version = "-"
		} else if useColors {
			version = versionStyle.Render(version)
		}

		handlerName := r.HandlerName
		if useColors && isBuiltin {
			handlerName = dimStyle.Render(handlerName)
		}

		rows = append(rows, []string{
			method,
			version,
			r.Path,
			handlerName,
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

	//nolint:errcheck,gosec // Best-effort table display; G705: t.Render() is server-generated route table, not user input
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
