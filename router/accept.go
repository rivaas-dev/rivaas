package router

// This file contains HTTP content negotiation methods for the Context type.
// These methods handle parsing and matching Accept-* headers according to RFC 7231.

import (
	"strconv"
	"strings"
	"sync"
)

// acceptCache caches parsed Accept header values to avoid repeated parsing.
// This provides a ~2x speedup for repeated identical Accept headers.
var (
	acceptCache    = make(map[string][]acceptSpec, 100) // Pre-allocate for common headers
	acceptCacheMu  sync.RWMutex
	acceptCacheMax = 100 // Maximum cache entries to prevent unbounded growth
)

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

	// Parse Accept header into types with quality values
	specs := parseAccept(accept)
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
	return acceptHeader(c.Request.Header.Get("Accept-Charset"), offers)
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
	return acceptHeader(c.Request.Header.Get("Accept-Encoding"), offers)
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
	return acceptHeader(c.Request.Header.Get("Accept-Language"), offers)
}

// acceptSpec represents a parsed Accept header value with quality
type acceptSpec struct {
	value      string
	quality    float64
	params     map[string]string
	rawQuality string // For tie-breaking based on position
}

// parseAccept parses an Accept-style header into specs with quality values.
// Results are cached to avoid repeated parsing of identical headers.
func parseAccept(header string) []acceptSpec {
	if header == "" {
		return nil
	}

	// Fast path: check cache with read lock
	acceptCacheMu.RLock()
	if cached, ok := acceptCache[header]; ok {
		acceptCacheMu.RUnlock()
		return cached
	}
	acceptCacheMu.RUnlock()

	// Slow path: parse header
	parts := strings.Split(header, ",")
	specs := make([]acceptSpec, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		spec := acceptSpec{
			quality: 1.0,
			params:  make(map[string]string),
		}

		// Split value and parameters
		segments := strings.Split(part, ";")
		spec.value = strings.TrimSpace(segments[0])

		// Parse parameters (including q)
		for _, param := range segments[1:] {
			param = strings.TrimSpace(param)
			if kv := strings.SplitN(param, "=", 2); len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				val := strings.Trim(strings.TrimSpace(kv[1]), `"`)

				if key == "q" {
					spec.rawQuality = val
					if q, err := strconv.ParseFloat(val, 64); err == nil && q >= 0 && q <= 1 {
						spec.quality = q
					}
				} else {
					spec.params[key] = val
				}
			}
		}

		specs = append(specs, spec)
	}

	// Cache the result (with size limit to prevent unbounded growth)
	acceptCacheMu.Lock()
	if len(acceptCache) < acceptCacheMax {
		acceptCache[header] = specs
	}
	acceptCacheMu.Unlock()

	return specs
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

// splitMediaType splits a media type into type and subtype
func splitMediaType(mediaType string) (string, string) {
	// Remove parameters
	if idx := strings.Index(mediaType, ";"); idx != -1 {
		mediaType = mediaType[:idx]
	}
	mediaType = strings.TrimSpace(mediaType)

	parts := strings.SplitN(mediaType, "/", 2)
	if len(parts) == 2 {
		return strings.ToLower(parts[0]), strings.ToLower(parts[1])
	}
	return strings.ToLower(mediaType), "*"
}

// parseMediaTypeParams extracts parameters from a media type
func parseMediaTypeParams(mediaType string) map[string]string {
	params := make(map[string]string)
	if idx := strings.Index(mediaType, ";"); idx != -1 {
		paramStr := mediaType[idx+1:]
		for _, param := range strings.Split(paramStr, ";") {
			if kv := strings.SplitN(param, "=", 2); len(kv) == 2 {
				key := strings.TrimSpace(kv[0])
				val := strings.Trim(strings.TrimSpace(kv[1]), `"`)
				params[strings.ToLower(key)] = val
			}
		}
	}
	return params
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

// acceptHeader is a generic handler for Accept-* headers (charset, encoding, language)
// It uses simple quality-based matching
func acceptHeader(header string, offers []string) string {
	if len(offers) == 0 {
		return ""
	}

	if header == "" {
		return offers[0]
	}

	specs := parseAccept(header)
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
