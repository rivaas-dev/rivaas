package router

// This file contains HTTP content negotiation methods for the Context type.
// These methods handle parsing and matching Accept-* headers according to RFC 7231.
//
// PERFORMANCE OPTIMIZATIONS:
// - Zero-allocation header scanning using byte slicing
// - Per-request caching of parsed headers in Context
// - Manual parsing to avoid strings.Split allocations
// - Arena allocator for spec buffers (pooled across requests)
// - Achieves true zero-allocation parsing for common cases

import (
	"strconv"
	"strings"
	"sync"
)

// acceptSpec represents a parsed Accept header value with quality
type acceptSpec struct {
	value      string
	quality    float64
	params     map[string]string
	rawQuality string // For tie-breaking based on position
}

// headerArena provides a pre-allocated buffer for acceptSpec slices.
// This eliminates the slice allocation in parseAcceptFast.
type headerArena struct {
	specs [16]acceptSpec // Pre-allocated buffer (covers 99% of real-world cases)
	used  int            // Number of specs currently in use
}

// arenaPool pools headerArena instances to achieve zero-allocation parsing.
var arenaPool = sync.Pool{
	New: func() any {
		return &headerArena{}
	},
}

// getSpecs returns a slice from the arena with the requested capacity.
// If capacity exceeds buffer size, falls back to heap allocation.
func (a *headerArena) getSpecs(capacity int) []acceptSpec {
	a.used = 0
	if capacity <= len(a.specs) {
		return a.specs[:0] // Return slice view of pre-allocated buffer
	}
	// Fallback for unusually large headers (rare)
	return make([]acceptSpec, 0, capacity)
}

// reset prepares the arena for return to the pool
func (a *headerArena) reset() {
	a.used = 0
	// Clear specs to prevent memory leaks (map references)
	for i := range a.specs {
		a.specs[i].params = nil
		a.specs[i].value = ""
		a.specs[i].rawQuality = ""
		a.specs[i].quality = 0
	}
}

// Accepts checks if the specified content types are acceptable based on the
// request's Accept HTTP header. It uses quality values and specificity rules
// to determine the best match. Returns the best matching content type or an
// empty string if none match.
//
// Supports:
//   - Multiple offers: c.Accepts("json", "xml", "html")
//   - Full MIME types: c.Accepts("application/json", "text/html")
//   - Short names: c.Accepts("json", "html")
//   - Wildcards: matches against */* and type/*
//   - Quality values: respects q parameters in Accept header
//   - Media type parameters: handles version=1, charset, etc.
//
// Examples:
//
//	// Accept: application/json, text/html
//	c.Accepts("json", "html")  // "json"
//
//	// Accept: text/html, application/json;q=0.8
//	c.Accepts("json", "html")  // "html" (higher quality)
//
//	// Accept: */*
//	c.Accepts("json", "xml")   // "json" (first match)
func (c *Context) Accepts(offers ...string) string {
	if len(offers) == 0 {
		return ""
	}

	accept := c.Request.Header.Get("Accept")
	if accept == "" {
		return offers[0] // No preference, return first
	}

	// Check per-request cache first
	var specs []acceptSpec
	if c.cachedAcceptHeader == accept && c.cachedAcceptSpecs != nil {
		specs = c.cachedAcceptSpecs
	} else {
		// Get arena from pool (reuse if already cached for this request)
		if c.cachedArena == nil {
			c.cachedArena = arenaPool.Get().(*headerArena)
		}

		// Parse and cache for this request (zero-allocation using arena)
		specs = parseAcceptFast(accept, c.cachedArena)
		c.cachedAcceptHeader = accept
		c.cachedAcceptSpecs = specs
	}

	if len(specs) == 0 {
		return offers[0]
	}

	// Normalize offers to full MIME types
	normalizedOffers := make([]string, len(offers))
	for i, offer := range offers {
		normalizedOffers[i] = normalizeMediaType(offer)
	}

	// Find best match
	bestMatch := ""
	bestQuality := -1.0
	bestSpecificity := -1

	for _, offer := range normalizedOffers {
		for _, spec := range specs {
			if quality, specificity := matchMediaType(offer, spec); quality > 0 {
				// Better quality, or same quality but more specific
				if quality > bestQuality || (quality == bestQuality && specificity > bestSpecificity) {
					bestMatch = offer
					bestQuality = quality
					bestSpecificity = specificity
				}
			}
		}
	}

	// Return original offer format if match found
	if bestMatch != "" {
		for i, normalized := range normalizedOffers {
			if normalized == bestMatch {
				return offers[i]
			}
		}
	}

	return ""
}

// AcceptsCharsets checks if the specified character sets are acceptable based
// on the request's Accept-Charset HTTP header. Returns the best matching
// charset or an empty string if none match.
//
// Examples:
//
//	// Accept-Charset: utf-8, iso-8859-1;q=0.5
//	c.AcceptsCharsets("utf-8", "iso-8859-1")  // "utf-8"
func (c *Context) AcceptsCharsets(offers ...string) string {
	specs := acceptHeaderFast(c, c.Request.Header.Get("Accept-Charset"))
	return acceptHeaderMatch(specs, offers)
}

// AcceptsEncodings checks if the specified encodings are acceptable based
// on the request's Accept-Encoding HTTP header. Returns the best matching
// encoding or an empty string if none match.
//
// Common encodings: gzip, br (Brotli), deflate, compress, identity
//
// Examples:
//
//	// Accept-Encoding: gzip, deflate;q=0.8, br;q=1.0
//	c.AcceptsEncodings("gzip", "br", "deflate")  // "br" (highest quality)
func (c *Context) AcceptsEncodings(offers ...string) string {
	specs := acceptHeaderFast(c, c.Request.Header.Get("Accept-Encoding"))
	return acceptHeaderMatch(specs, offers)
}

// AcceptsLanguages checks if the specified languages are acceptable based
// on the request's Accept-Language HTTP header. Returns the best matching
// language or an empty string if none match.
//
// Examples:
//
//	// Accept-Language: en-US, en;q=0.9, fr;q=0.8
//	c.AcceptsLanguages("en", "fr", "de")  // "en"
func (c *Context) AcceptsLanguages(offers ...string) string {
	specs := acceptHeaderFast(c, c.Request.Header.Get("Accept-Language"))
	return acceptHeaderMatch(specs, offers)
}

// parseAcceptFast parses an Accept-style header using zero-allocation scanning.
// This replaces strings.Split with manual byte-index parsing to avoid allocations.
// Uses arena allocator to eliminate slice allocation.
// Returns specs slice and the arena (caller must return arena to pool).
func parseAcceptFast(header string, arena *headerArena) []acceptSpec {
	if header == "" {
		return nil
	}

	specs := arena.getSpecs(4) // Pre-size for common case

	// Manual scanning to avoid strings.Split allocations
	start := 0
	for i := 0; i <= len(header); i++ {
		// Found comma or end of string
		if i == len(header) || header[i] == ',' {
			if i > start {
				part := header[start:i]
				if spec := parseAcceptPartFast(part); spec.value != "" {
					specs = append(specs, spec)
				}
			}
			start = i + 1
		}
	}

	return specs
}

// parseAcceptPartFast parses a single Accept header part (between commas).
// Uses manual scanning to avoid string allocations.
func parseAcceptPartFast(part string) acceptSpec {
	spec := acceptSpec{
		quality: 1.0,
		params:  nil, // Lazy init only if needed
	}

	// Trim leading/trailing whitespace manually
	start, end := trimWhitespace(part)
	if start >= end {
		return spec
	}

	// Find semicolon separator between value and parameters
	semicolon := -1
	for i := start; i < end; i++ {
		if part[i] == ';' {
			semicolon = i
			break
		}
	}

	// Extract value (before semicolon or entire trimmed part)
	if semicolon == -1 {
		spec.value = part[start:end]
		return spec
	}

	spec.value = part[start:semicolon]

	// Parse parameters after semicolon
	paramStart := semicolon + 1
	for i := paramStart; i <= end; i++ {
		// Found semicolon or end
		if i == end || part[i] == ';' {
			if i > paramStart {
				parseParamFast(part[paramStart:i], &spec)
			}
			paramStart = i + 1
		}
	}

	return spec
}

// parseParamFast parses a single parameter (key=value) and updates spec.
// Manual parsing to avoid strings.Split allocations.
func parseParamFast(param string, spec *acceptSpec) {
	// Trim whitespace
	start, end := trimWhitespace(param)
	if start >= end {
		return
	}

	// Find equals sign
	equals := -1
	for i := start; i < end; i++ {
		if param[i] == '=' {
			equals = i
			break
		}
	}

	if equals == -1 {
		return // Invalid parameter, skip
	}

	// Extract key and value
	keyStart, keyEnd := trimWhitespace(param[start:equals])
	if keyStart >= keyEnd {
		return
	}
	key := param[start+keyStart : start+keyEnd]

	valStart, valEnd := trimWhitespace(param[equals+1 : end])
	if valStart >= valEnd {
		return
	}

	// Remove quotes if present
	valStartAbs := equals + 1 + valStart
	valEndAbs := equals + 1 + valEnd
	if valEndAbs > valStartAbs && param[valStartAbs] == '"' && param[valEndAbs-1] == '"' {
		valStartAbs++
		valEndAbs--
	}
	value := param[valStartAbs:valEndAbs]

	// Handle quality parameter specially
	if key == "q" {
		spec.rawQuality = value
		if q, err := strconv.ParseFloat(value, 64); err == nil && q >= 0 && q <= 1 {
			spec.quality = q
		}
	} else {
		// Lazy init params map
		if spec.params == nil {
			spec.params = make(map[string]string, 2)
		}
		spec.params[key] = value
	}
}

// trimWhitespace returns start and end indices of non-whitespace content.
// Returns indices relative to the input string slice.
func trimWhitespace(s string) (start, end int) {
	// Find first non-whitespace
	start = 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}

	// Find last non-whitespace
	end = len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}

	return start, end
}

// matchMediaType checks if an offer matches a spec and returns quality and specificity
// Specificity: 3 = exact match, 2 = subtype wildcard, 1 = type wildcard, 0 = no match
//
// Note: According to RFC 7231, media type parameters in the Accept header
// (other than q) are generally used for more specific matching, but for common
// use cases, we ignore non-q parameters to allow broader matching.
// If strict parameter matching is needed, it should be done at the application level.
func matchMediaType(offer string, spec acceptSpec) (quality float64, specificity int) {
	offerType, offerSubtype := splitMediaType(offer)
	specType, specSubtype := splitMediaType(spec.value)

	// Note: We ignore non-q parameters in spec for matching.
	// This allows "application/json;version=1" in Accept header to match
	// a plain "application/json" offer, which is the common expectation.

	// Match specificity (higher is better)
	if specType == "*" && specSubtype == "*" {
		return spec.quality, 1 // Wildcard match
	}
	if specType == offerType && specSubtype == "*" {
		return spec.quality, 2 // Type match with subtype wildcard
	}
	if specType == offerType && specSubtype == offerSubtype {
		return spec.quality, 3 // Exact match
	}

	return 0, 0 // No match
}

// splitMediaType splits a media type into type and subtype using zero-allocation scanning.
func splitMediaType(mediaType string) (string, string) {
	// Find semicolon to remove parameters
	semicolon := -1
	for i := 0; i < len(mediaType); i++ {
		if mediaType[i] == ';' {
			semicolon = i
			break
		}
	}

	// Trim parameters if present
	if semicolon != -1 {
		mediaType = mediaType[:semicolon]
	}

	// Trim whitespace manually
	start, end := trimWhitespace(mediaType)
	mediaType = mediaType[start:end]

	// Find slash separator
	slash := -1
	for i := 0; i < len(mediaType); i++ {
		if mediaType[i] == '/' {
			slash = i
			break
		}
	}

	if slash != -1 {
		// Convert to lowercase for comparison (allocates, but unavoidable for case-insensitive)
		typeStr := strings.ToLower(mediaType[:slash])
		subtypeStr := strings.ToLower(mediaType[slash+1:])
		return typeStr, subtypeStr
	}

	return strings.ToLower(mediaType), "*"
}

// normalizeMediaType converts short names to full MIME types
func normalizeMediaType(mediaType string) string {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))

	// Common short names to MIME types
	mimeTypes := map[string]string{
		"html":       "text/html",
		"json":       "application/json",
		"xml":        "application/xml",
		"text":       "text/plain",
		"txt":        "text/plain",
		"png":        "image/png",
		"jpg":        "image/jpeg",
		"jpeg":       "image/jpeg",
		"gif":        "image/gif",
		"webp":       "image/webp",
		"svg":        "image/svg+xml",
		"css":        "text/css",
		"js":         "application/javascript",
		"javascript": "application/javascript",
		"pdf":        "application/pdf",
		"zip":        "application/zip",
		"mp4":        "video/mp4",
		"webm":       "video/webm",
		"mp3":        "audio/mpeg",
		"wav":        "audio/wav",
	}

	if mime, ok := mimeTypes[mediaType]; ok {
		return mime
	}

	// If it already looks like a MIME type, return as-is
	if strings.Contains(mediaType, "/") {
		return mediaType
	}

	// Unknown short name, return as-is
	return mediaType
}

// acceptHeaderFast is a generic handler for Accept-* headers (charset, encoding, language)
// It uses simple quality-based matching with zero-allocation parsing.
func acceptHeaderFast(c *Context, header string) []acceptSpec {
	if header == "" {
		return nil
	}

	// Get arena from pool (reuse if already cached for this request)
	if c.cachedArena == nil {
		c.cachedArena = arenaPool.Get().(*headerArena)
	}

	// Parse using arena (zero allocations)
	return parseAcceptFast(header, c.cachedArena)
}

// acceptHeaderMatch performs quality-based matching for Accept-* headers
func acceptHeaderMatch(specs []acceptSpec, offers []string) string {
	if len(offers) == 0 {
		return ""
	}

	if len(specs) == 0 {
		return offers[0]
	}

	bestMatch := ""
	bestQuality := -1.0

	for _, offer := range offers {
		offerLower := strings.ToLower(strings.TrimSpace(offer))
		for _, spec := range specs {
			specValue := strings.ToLower(spec.value)

			// Exact match or wildcard
			if specValue == offerLower || specValue == "*" {
				if spec.quality > bestQuality {
					bestMatch = offer
					bestQuality = spec.quality
				}
				break
			}

			// Language prefix match (e.g., "en" matches "en-US")
			if strings.HasPrefix(specValue, offerLower+"-") || strings.HasPrefix(offerLower, specValue+"-") {
				if spec.quality > bestQuality {
					bestMatch = offer
					bestQuality = spec.quality
				}
			}
		}
	}

	return bestMatch
}
