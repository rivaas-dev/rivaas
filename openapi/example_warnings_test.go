package openapi_test

import (
	"context"
	"fmt"

	"rivaas.dev/openapi"
	"rivaas.dev/openapi/diag"
)

// Example_warnings demonstrates how to work with warnings from spec generation.
func Example_warnings() {
	// Create API targeting OpenAPI 3.0 with 3.1-only features
	api := openapi.MustNew(
		openapi.WithTitle("My API", "1.0.0"),
		openapi.WithVersion(openapi.V30x),
		openapi.WithInfoSummary("A modern API"), // 3.1-only feature
	)

	result, err := api.Generate(context.Background(),
		openapi.GET("/health", openapi.WithResponse(200, map[string]string{})),
	)
	if err != nil {
		panic(err)
	}

	// Simple warning check
	if len(result.Warnings) > 0 {
		fmt.Printf("Generated with %d warnings\n", len(result.Warnings))
	}

	// Type-safe warning check (requires diag import)
	if result.Warnings.Has(diag.WarnDownlevelInfoSummary) {
		fmt.Println("Info summary was dropped for OpenAPI 3.0 compatibility")
	}

	// Filter by category
	downlevelWarnings := result.Warnings.FilterCategory(diag.CategoryDownlevel)
	fmt.Printf("Downlevel warnings: %d\n", len(downlevelWarnings))

	// Process warnings
	result.Warnings.Each(func(w diag.Warning) {
		fmt.Printf("[%s] %s\n", w.Code(), w.Message())
	})

	// Output:
	// Generated with 1 warnings
	// Info summary was dropped for OpenAPI 3.0 compatibility
	// Downlevel warnings: 1
	// [DOWNLEVEL_INFO_SUMMARY] info.summary is 3.1-only; dropped
}

// Example_warningsStrictMode demonstrates strict downlevel mode.
func Example_warningsStrictMode() {
	// Strict mode treats downlevel issues as errors
	api := openapi.MustNew(
		openapi.WithTitle("API", "1.0.0"),
		openapi.WithVersion(openapi.V30x),
		openapi.WithStrictDownlevel(true),  // Errors instead of warnings
		openapi.WithInfoSummary("Summary"), // 3.1-only feature
	)

	_, err := api.Generate(context.Background(),
		openapi.GET("/health", openapi.WithResponse(200, map[string]string{})),
	)
	// In strict mode, using 3.1 features with 3.0 target returns an error
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Output:
	// Error: failed to project OpenAPI spec: info.summary not supported in OpenAPI 3.0
}

// Example_warningsFiltering demonstrates advanced warning filtering.
func Example_warningsFiltering() {
	api := openapi.MustNew(
		openapi.WithTitle("API", "1.0.0"),
		openapi.WithVersion(openapi.V30x),
		openapi.WithInfoSummary("Summary"),            // 3.1 feature
		openapi.WithLicenseIdentifier("MIT", "MIT-0"), // 3.1 feature
	)

	result, err := api.Generate(context.Background(),
		openapi.GET("/health", openapi.WithResponse(200, map[string]string{})),
	)
	if err != nil {
		panic(err)
	}

	// Get only specific warnings
	licenseWarnings := result.Warnings.Filter(diag.WarnDownlevelLicenseIdentifier)
	fmt.Printf("License warnings: %d\n", len(licenseWarnings))

	// Exclude expected warnings
	unexpected := result.Warnings.Exclude(diag.WarnDownlevelInfoSummary)
	fmt.Printf("Unexpected warnings: %d\n", len(unexpected))

	// Check for any of multiple codes
	hasSecurityIssues := result.Warnings.HasAny(
		diag.WarnDownlevelMutualTLS,
		diag.WarnDownlevelWebhooks,
	)
	fmt.Printf("Has security issues: %v\n", hasSecurityIssues)

	// Output:
	// License warnings: 1
	// Unexpected warnings: 1
	// Has security issues: false
}
