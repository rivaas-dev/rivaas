package router

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"strings"

	"rivaas.dev/router/binding"
	"rivaas.dev/router/validation"
)

// bindingMetadata holds per-request binding state.
type bindingMetadata struct {
	bodyRead bool                   // Whether the body has been read
	rawBody  []byte                 // Cached raw body bytes
	presence validation.PresenceMap // Tracks which fields are present in the request
}

// BindBody binds the request body to a struct based on the Content-Type header.
//
// Supported content types:
//   - application/json (uses json struct tags)
//   - application/x-www-form-urlencoded (uses form struct tags)
//   - multipart/form-data (uses form struct tags)
//
// For JSON, it uses the standard encoding/json package.
// For forms, it parses form data and binds using reflection.
//
// Note: For multipart forms with file uploads, files must be retrieved
// separately using c.Request.FormFile() or c.Request.MultipartForm.
func (c *Context) BindBody(out any) error {
	contentType := c.Request.Header.Get("Content-Type")

	// Extract base content type (remove parameters)
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(strings.ToLower(contentType))

	switch contentType {
	case "application/json", "application/merge-patch+json", "application/json-patch+json", "":
		// Default to JSON if no content type specified
		return c.BindJSON(out)
	case "application/x-www-form-urlencoded":
		return c.BindForm(out)
	case "multipart/form-data":
		return c.BindForm(out)
	default:
		return fmt.Errorf("%w: %s", binding.ErrUnsupportedContentType, contentType)
	}
}

// BindJSON binds JSON request body to a struct.
// Uses json struct tags.
//
// Example:
//
//	type User struct {
//	    Name  string `json:"name"`
//	    Email string `json:"email"`
//	}
//
//	var user User
//	if err := c.BindJSON(&user); err != nil {
//	    return err
//	}
func (c *Context) BindJSON(out any) error {
	if c.Request.Body == nil {
		return binding.ErrRequestBodyNil
	}

	// Read and cache body
	if c.bindingMeta == nil {
		c.bindingMeta = &bindingMetadata{}
	}

	if !c.bindingMeta.bodyRead {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return fmt.Errorf("failed to read body: %w", err)
		}
		c.bindingMeta.rawBody = body
		c.bindingMeta.bodyRead = true

		// Refill for downstream middleware
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		// Track presence using validation package
		if pm, err := validation.ComputePresence(body); err == nil {
			c.bindingMeta.presence = pm
		}

		// Store raw JSON in context for schema validation optimization
		c.Request = c.Request.WithContext(
			validation.InjectRawJSONCtx(c.Request.Context(), body),
		)
	}

	return binding.BindJSONBytes(out, c.bindingMeta.rawBody)
}

// BindJSONStrict binds JSON request body with unknown field rejection.
// This is useful for catching typos and API drift early.
//
// Example:
//
//	var user User
//	if err := c.BindJSONStrict(&user); err != nil {
//	    // Returns error if JSON contains unknown fields
//	    return err
//	}
func (c *Context) BindJSONStrict(out any) error {
	if c.Request.Body == nil {
		return binding.ErrRequestBodyNil
	}

	// Read and cache body (same as BindJSON)
	if c.bindingMeta == nil {
		c.bindingMeta = &bindingMetadata{}
	}

	if !c.bindingMeta.bodyRead {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return fmt.Errorf("failed to read body: %w", err)
		}
		c.bindingMeta.rawBody = body
		c.bindingMeta.bodyRead = true

		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		// Track presence using validation package
		if pm, err := validation.ComputePresence(body); err == nil {
			c.bindingMeta.presence = pm
		}

		c.Request = c.Request.WithContext(
			validation.InjectRawJSONCtx(c.Request.Context(), body),
		)
	}

	// Use strict binding
	err := binding.BindJSONStrictBytes(out, c.bindingMeta.rawBody)

	// Translate binding.UnknownFieldError to validation.Error (only here!)
	if unkErr, ok := err.(*binding.UnknownFieldError); ok {
		return &validation.Error{
			Fields: []validation.FieldError{{
				Code:    "json.unknown_field",
				Message: unkErr.Error(),
			}},
		}
	}

	return err
}

// Presence returns the presence map for the current request.
// Returns nil if no binding has occurred yet.
func (c *Context) Presence() validation.PresenceMap {
	if c.bindingMeta == nil {
		return nil
	}
	return c.bindingMeta.presence
}

// ResetBinding resets the binding metadata for this context.
// Useful for testing or when you need to rebind a request.
func (c *Context) ResetBinding() {
	c.bindingMeta = nil
}

// BindForm binds form data to a struct.
// Handles both application/x-www-form-urlencoded and multipart/form-data.
// Uses form struct tags.
func (c *Context) BindForm(out any) error {
	contentType := c.Request.Header.Get("Content-Type")

	// Parse the appropriate form type
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil { // 32 MB max
			return fmt.Errorf("failed to parse multipart form: %w", err)
		}
	} else {
		if err := c.Request.ParseForm(); err != nil {
			return fmt.Errorf("failed to parse form: %w", err)
		}
	}

	return binding.Bind(out, binding.NewFormGetter(c.Request.Form), binding.TagForm)
}

// BindQuery binds query parameters to a struct.
// Uses query struct tags.
func (c *Context) BindQuery(out any) error {
	return binding.Bind(out, binding.NewQueryGetter(c.Request.URL.Query()), binding.TagQuery)
}

// BindParams binds URL path parameters to a struct.
// Uses params struct tags.
func (c *Context) BindParams(out any) error {
	// Build params map from both array-based params and map-based params
	allParams := make(map[string]string, c.paramCount)

	// Copy from array (fast path for ≤8 params)
	for i := range c.paramCount {
		allParams[c.paramKeys[i]] = c.paramValues[i]
	}

	// Copy from map (fallback for >8 params)
	maps.Copy(allParams, c.Params)

	return binding.Bind(out, binding.NewParamsGetter(allParams), binding.TagParams)
}

// BindCookies binds cookies to a struct.
// Uses cookie struct tags. Cookie values are automatically URL-unescaped.
func (c *Context) BindCookies(out any) error {
	return binding.Bind(out, binding.NewCookieGetter(c.Request.Cookies()), binding.TagCookie)
}

// BindHeaders binds HTTP headers to a struct.
// Uses header struct tags. Header names are case-insensitive.
func (c *Context) BindHeaders(out any) error {
	return binding.Bind(out, binding.NewHeaderGetter(c.Request.Header), binding.TagHeader)
}
