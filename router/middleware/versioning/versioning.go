// Package versioning provides middleware for API versioning support,
// allowing routes to be versioned based on headers, query parameters, or paths.
package versioning

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"rivaas.dev/router"
)

// VersionInfo describes a single API version with deprecation and sunset information.
type VersionInfo struct {
	Version    string     // Version string (e.g., "v1", "v2")
	Deprecated *time.Time // When version becomes deprecated (advisory)
	Sunset     *time.Time // When version will be/has been removed (final)
	DocsURL    string     // Optional documentation URL
}

// Options configures the versioning middleware.
type Options struct {
	Versions          []VersionInfo                                    // Version information
	Now               func() time.Time                                 // Injectable clock for tests
	EmitWarning299    bool                                             // Emit Warning: 299 deprecation text
	OnDeprecatedUse   func(ctx context.Context, version, route string) // Callback for deprecated API usage
	SendVersionHeader bool                                             // Add X-API-Version header on responses
}

// ValidateVersions validates version configuration and returns an error if invalid.
// Ensures Sunset >= Deprecated when both are set, and checks for duplicate versions.
func ValidateVersions(vs []VersionInfo) error {
	seen := make(map[string]bool)

	for _, v := range vs {
		if v.Version == "" {
			return fmt.Errorf("version string cannot be empty")
		}

		if seen[v.Version] {
			return fmt.Errorf("duplicate version: %s", v.Version)
		}
		seen[v.Version] = true

		if v.Deprecated != nil && v.Sunset != nil {
			if v.Sunset.Before(*v.Deprecated) {
				return fmt.Errorf(
					"version %s: sunset (%s) must be after deprecated (%s)",
					v.Version,
					v.Sunset.Format(time.RFC3339),
					v.Deprecated.Format(time.RFC3339),
				)
			}
		}
	}

	return nil
}

// WithVersioning creates a middleware that handles API version deprecation and sunset headers.
//
// This middleware should be applied after version detection middleware/router.
// It checks the detected version against the configured VersionInfo and sets appropriate headers.
//
// Example:
//
//	v1Dep := time.Date(2025,1,1,0,0,0,time.UTC)
//	v1Sun := time.Date(2025,4,1,0,0,0,time.UTC)
//	r.Use(versioning.WithVersioning(versioning.Options{
//	    Versions: []versioning.VersionInfo{
//	        {Version:"v1", Deprecated:&v1Dep, Sunset:&v1Sun, DocsURL:"https://docs.rivaas.dev/v1"},
//	        {Version:"v2"},
//	    },
//	    SendVersionHeader: true,
//	    EmitWarning299: true,
//	    OnDeprecatedUse: func(ctx context.Context, v, route string) {
//	        metrics.DeprecatedAPIUsage.WithLabels(v, route).Inc()
//	    },
//	    Now: time.Now,
//	}))
func WithVersioning(opts Options) router.HandlerFunc {
	// Apply defaults
	if opts.Now == nil {
		opts.Now = time.Now
	}

	// Validate versions at startup
	if err := ValidateVersions(opts.Versions); err != nil {
		panic(fmt.Sprintf("versioning: invalid version configuration: %v", err))
	}

	// Build version lookup map
	versionMap := make(map[string]VersionInfo, len(opts.Versions))
	for _, v := range opts.Versions {
		versionMap[v.Version] = v
	}

	return func(c *router.Context) {
		// Get detected version from context
		version := c.Version()
		if version == "" {
			// No version detected, skip
			c.Next()
			return
		}

		// Look up version info
		info, exists := versionMap[version]
		if !exists {
			// Unknown version, skip
			c.Next()
			return
		}

		now := opts.Now()

		// Send X-API-Version header if enabled
		if opts.SendVersionHeader {
			c.Header("X-API-Version", version)
		}

		// Check if past sunset date
		if info.Sunset != nil && now.After(*info.Sunset) {
			// Version has been removed - return 410 Gone
			c.WriteErrorResponse(http.StatusGone, fmt.Sprintf("API %s was removed on %s. Please upgrade to a supported version.", version, info.Sunset.Format(time.RFC3339)))
			c.Abort()
			return
		}

		// Check if deprecated
		if info.Deprecated != nil && !now.Before(*info.Deprecated) {
			// Version is deprecated

			// Set Deprecation header (draft-ietf-httpapi-deprecation-header)
			// Format: HTTP-date (always use HTTP-date format for deprecated versions)
			c.Header("Deprecation", info.Deprecated.UTC().Format(http.TimeFormat))

			// Set Link header with deprecation relation
			if info.DocsURL != "" {
				c.Header("Link", fmt.Sprintf("<%s>; rel=\"deprecation\"", info.DocsURL))
			}

			// Optionally emit Warning: 299
			if opts.EmitWarning299 {
				sunsetText := ""
				if info.Sunset != nil {
					sunsetText = fmt.Sprintf(" and will be removed on %s", info.Sunset.Format(time.RFC3339))
				}
				warning := fmt.Sprintf("299 - \"API %s is deprecated%s. Please upgrade to a supported version.\"", version, sunsetText)
				c.Header("Warning", warning)
			}

			// Call callback for deprecated usage
			if opts.OnDeprecatedUse != nil {
				// Call asynchronously to avoid blocking request
				go opts.OnDeprecatedUse(c.Request.Context(), version, c.RouteTemplate())
			}
		}

		// Always set Sunset header if sunset date is configured (RFC 8594)
		if info.Sunset != nil {
			c.Header("Sunset", info.Sunset.UTC().Format(http.TimeFormat))

			// Set Link header with sunset relation
			if info.DocsURL != "" {
				// Append to existing Link header if present
				existingLink := c.Response.Header().Get("Link")
				sunsetLink := fmt.Sprintf("<%s>; rel=\"sunset\"", info.DocsURL)
				if existingLink != "" {
					c.Header("Link", existingLink+", "+sunsetLink)
				} else {
					c.Header("Link", sunsetLink)
				}
			}
		}

		c.Next()
	}
}
