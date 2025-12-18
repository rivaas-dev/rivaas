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

package openapi

// UIOption configures Swagger UI behavior and appearance.
type UIOption func(*UIConfig)

// WithSwaggerUI enables Swagger UI at the given path with optional configuration.
//
// The path parameter specifies where the Swagger UI will be served (e.g., "/docs").
// UI options can be provided to customize the appearance and behavior.
//
// Example:
//
//	openapi.MustNew(
//	    openapi.WithTitle("API", "1.0.0"),
//	    openapi.WithSwaggerUI("/docs",
//	        openapi.WithUIExpansion(openapi.DocExpansionList),
//	        openapi.WithUITryItOut(true),
//	        openapi.WithUISyntaxTheme(openapi.SyntaxThemeMonokai),
//	    ),
//	)
func WithSwaggerUI(path string, opts ...UIOption) Option {
	return func(a *API) {
		a.ServeUI = true
		a.UIPath = path
		for _, opt := range opts {
			opt(&a.ui)
		}
	}
}

// WithoutSwaggerUI disables Swagger UI serving.
//
// Example:
//
//	openapi.MustNew(
//	    openapi.WithTitle("API", "1.0.0"),
//	    openapi.WithoutSwaggerUI(),
//	)
func WithoutSwaggerUI() Option {
	return func(a *API) {
		a.ServeUI = false
	}
}

// WithUIDeepLinking enables or disables deep linking in Swagger UI.
//
// When enabled, Swagger UI updates the browser URL when operations are expanded,
// allowing direct linking to specific operations. Default: true.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIDeepLinking(true),
//	)
func WithUIDeepLinking(enabled bool) UIOption {
	return func(c *UIConfig) {
		c.DeepLinking = enabled
	}
}

// WithUIDisplayOperationID shows or hides operation IDs in Swagger UI.
//
// Operation IDs are useful for code generation and API client libraries.
// Default: false.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIDisplayOperationID(true),
//	)
func WithUIDisplayOperationID(show bool) UIOption {
	return func(c *UIConfig) {
		c.DisplayOperationID = show
	}
}

// WithUIExpansion sets the default expansion level for operations and tags.
//
// Valid modes:
//   - DocExpansionList: Expand only tags (default)
//   - DocExpansionFull: Expand tags and operations
//   - DocExpansionNone: Collapse everything
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIExpansion(openapi.DocExpansionFull),
//	)
func WithUIExpansion(mode DocExpansionMode) UIOption {
	return func(c *UIConfig) {
		c.DocExpansion = mode
	}
}

// WithUIModelsExpandDepth sets the default expansion depth for model schemas.
//
// Depth controls how many levels of nested properties are expanded by default.
// Use -1 to hide models completely. Default: 1.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIModelsExpandDepth(2), // Expand 2 levels deep
//	)
func WithUIModelsExpandDepth(depth int) UIOption {
	return func(c *UIConfig) {
		c.DefaultModelsExpandDepth = depth
	}
}

// WithUIModelExpandDepth sets the default expansion depth for model example sections.
//
// Controls how many levels of the example value are expanded. Default: 1.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIModelExpandDepth(3),
//	)
func WithUIModelExpandDepth(depth int) UIOption {
	return func(c *UIConfig) {
		c.DefaultModelExpandDepth = depth
	}
}

// WithUIDefaultModelRendering sets the initial model display mode.
//
// Valid modes:
//   - ModelRenderingExample: Show example value (default)
//   - ModelRenderingModel: Show model structure
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIDefaultModelRendering(openapi.ModelRenderingModel),
//	)
func WithUIDefaultModelRendering(mode ModelRenderingMode) UIOption {
	return func(c *UIConfig) {
		c.DefaultModelRendering = mode
	}
}

// WithUITryItOut enables or disables "Try it out" functionality by default.
//
// When enabled, the "Try it out" button is automatically expanded for all operations.
// Default: true.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUITryItOut(false), // Require users to click "Try it out"
//	)
func WithUITryItOut(enabled bool) UIOption {
	return func(c *UIConfig) {
		c.TryItOutEnabled = enabled
	}
}

// WithUIRequestSnippets enables or disables code snippet generation.
//
// When enabled, Swagger UI generates code snippets showing how to call the API
// in various languages (curl, etc.). The languages parameter specifies which
// snippet generators to include. If not provided, defaults to curl_bash.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIRequestSnippets(true, openapi.SnippetCurlBash, openapi.SnippetCurlPowerShell),
//	)
func WithUIRequestSnippets(enabled bool, languages ...RequestSnippetLanguage) UIOption {
	return func(c *UIConfig) {
		c.RequestSnippetsEnabled = enabled
		if len(languages) > 0 {
			c.RequestSnippets.Languages = languages
		}
	}
}

// WithUIRequestSnippetsExpanded sets whether request snippets are expanded by default.
//
// When true, code snippets are shown immediately without requiring user interaction.
// Default: false.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIRequestSnippetsExpanded(true),
//	)
func WithUIRequestSnippetsExpanded(expanded bool) UIOption {
	return func(c *UIConfig) {
		c.RequestSnippets.DefaultExpanded = expanded
	}
}

// WithUIDisplayRequestDuration shows or hides request duration in Swagger UI.
//
// When enabled, the time taken for "Try it out" requests is displayed.
// Default: true.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIDisplayRequestDuration(true),
//	)
func WithUIDisplayRequestDuration(show bool) UIOption {
	return func(c *UIConfig) {
		c.DisplayRequestDuration = show
	}
}

// WithUIFilter enables or disables the operation filter/search box.
//
// When enabled, users can filter operations by typing in a search box.
// Default: true.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIFilter(true),
//	)
func WithUIFilter(enabled bool) UIOption {
	return func(c *UIConfig) {
		c.Filter = enabled
	}
}

// WithUIMaxDisplayedTags limits the number of tags displayed in Swagger UI.
//
// When set to a positive number, only the first N tags are shown. Remaining tags
// are hidden. Use 0 or negative to show all tags. Default: 0 (show all).
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIMaxDisplayedTags(10), // Show only first 10 tags
//	)
func WithUIMaxDisplayedTags(max int) UIOption {
	return func(c *UIConfig) {
		c.MaxDisplayedTags = max
	}
}

// WithUIOperationsSorter sets how operations are sorted within tags.
//
// Valid modes:
//   - OperationsSorterAlpha: Sort alphabetically by path
//   - OperationsSorterMethod: Sort by HTTP method (GET, POST, etc.)
//   - OperationsSorterNone: Use server order (no sorting, default)
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIOperationsSorter(openapi.OperationsSorterAlpha),
//	)
func WithUIOperationsSorter(mode OperationsSorterMode) UIOption {
	return func(c *UIConfig) {
		c.OperationsSorter = mode
	}
}

// WithUITagsSorter sets how tags are sorted in Swagger UI.
//
// Valid modes:
//   - TagsSorterAlpha: Sort tags alphabetically
//   - TagsSorterNone: Use server order (no sorting, default)
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUITagsSorter(openapi.TagsSorterAlpha),
//	)
func WithUITagsSorter(mode TagsSorterMode) UIOption {
	return func(c *UIConfig) {
		c.TagsSorter = mode
	}
}

// WithUISyntaxHighlight enables or disables syntax highlighting in Swagger UI.
//
// When enabled, request/response examples and code snippets are syntax-highlighted
// using the configured theme. Default: true.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUISyntaxHighlight(true),
//	)
func WithUISyntaxHighlight(enabled bool) UIOption {
	return func(c *UIConfig) {
		c.SyntaxHighlight.Activated = enabled
	}
}

// WithUISyntaxTheme sets the syntax highlighting theme for code examples.
//
// Available themes: Agate, Arta, Monokai, Nord, Obsidian, TomorrowNight, Idea.
// Default: Agate.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUISyntaxTheme(openapi.SyntaxThemeMonokai),
//	)
func WithUISyntaxTheme(theme SyntaxTheme) UIOption {
	return func(c *UIConfig) {
		c.SyntaxHighlight.Theme = theme
	}
}

// WithUIValidator sets the OpenAPI specification validator URL.
//
// Swagger UI can validate your OpenAPI spec against a validator service.
// Options:
//   - ValidatorLocal ("local"): Validate locally using embedded meta-schema (recommended)
//     No external calls, fast, private, works offline
//   - ValidatorNone ("none") or "": Disable validation
//   - URL string: Use an external validator service (e.g., "https://validator.swagger.io/validator")
//
// Default: "" (no validation)
//
// Example:
//
//	// Use local validation (recommended)
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIValidator(openapi.ValidatorLocal),
//	)
//
//	// Use external validator
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIValidator("https://validator.swagger.io/validator"),
//	)
//
//	// Disable validation
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIValidator(openapi.ValidatorNone),
//	)
func WithUIValidator(url string) UIOption {
	return func(c *UIConfig) {
		c.ValidatorURL = url
	}
}

// WithUIPersistAuth enables or disables authorization persistence.
//
// When enabled, authorization tokens are persisted in browser storage and
// automatically included in subsequent requests. Default: true.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIPersistAuth(true),
//	)
func WithUIPersistAuth(enabled bool) UIOption {
	return func(c *UIConfig) {
		c.PersistAuthorization = enabled
	}
}

// WithUIWithCredentials enables or disables credentials in CORS requests.
//
// When enabled, cookies and authorization headers are included in cross-origin
// requests. Only enable if your API server is configured to accept credentials.
// Default: false.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIWithCredentials(true),
//	)
func WithUIWithCredentials(enabled bool) UIOption {
	return func(c *UIConfig) {
		c.WithCredentials = enabled
	}
}

// WithUISupportedMethods sets which HTTP methods have "Try it out" enabled.
//
// By default, all standard HTTP methods support "Try it out". Use this option
// to restrict which methods can be tested interactively in Swagger UI.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUISupportedMethods(openapi.MethodGet, openapi.MethodPost),
//	)
func WithUISupportedMethods(methods ...HTTPMethod) UIOption {
	return func(c *UIConfig) {
		c.SupportedSubmitMethods = methods
	}
}

// WithUIShowExtensions shows or hides vendor extensions (x-* fields) in Swagger UI.
//
// Vendor extensions are custom fields prefixed with "x-" in the OpenAPI spec.
// Default: false.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIShowExtensions(true),
//	)
func WithUIShowExtensions(show bool) UIOption {
	return func(c *UIConfig) {
		c.ShowExtensions = show
	}
}

// WithUIShowCommonExtensions shows or hides common JSON Schema extensions.
//
// When enabled, displays schema constraints like pattern, maxLength, minLength,
// etc. in the UI. Default: true.
//
// Example:
//
//	openapi.WithSwaggerUI("/docs",
//	    openapi.WithUIShowCommonExtensions(true),
//	)
func WithUIShowCommonExtensions(show bool) UIOption {
	return func(c *UIConfig) {
		c.ShowCommonExtensions = show
	}
}
