package router

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"sync"
	"time"
)

// MediaTypeProblemJSON is the RFC 9457 content type for problem details.
// Note: RFC 9457 doesn't mandate charset=utf-8, but it's harmless and explicit.
const MediaTypeProblemJSON = "application/problem+json; charset=utf-8"

// bufferPool reduces allocations for frequent problem responses (validation, rate limits).
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// generateErrorID generates a unique error ID for correlation.
// Uses the same approach as requestid middleware for consistency.
func generateErrorID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback: use timestamp-based ID if random fails
		return fmt.Sprintf("err-%d", time.Now().UnixNano())
	}
	return "err-" + hex.EncodeToString(bytes)
}

// ProblemDetail represents an RFC 9457 Problem Details for HTTP APIs response.
// See: https://datatracker.ietf.org/doc/html/rfc9457 (obsoletes RFC 7807)
//
// Supports error chaining via Unwrap() for integration with errors.Is/As.
type ProblemDetail struct {
	// Type is a URI reference that identifies the problem type.
	// When dereferenced, it SHOULD provide human-readable documentation.
	// Defaults to "about:blank" if not set.
	Type string `json:"type"`

	// Title is a short, human-readable summary of the problem type.
	// It SHOULD NOT change from occurrence to occurrence, except for localization.
	Title string `json:"title"`

	// Status is the HTTP status code for this occurrence of the problem.
	Status int `json:"status"`

	// Detail is a human-readable explanation specific to this occurrence.
	Detail string `json:"detail,omitempty"`

	// Instance is a URI reference that identifies the specific occurrence.
	// It may or may not yield further information if dereferenced.
	Instance string `json:"instance,omitempty"`

	// Extensions contains additional problem-specific fields.
	// These will be marshaled inline with the standard fields.
	Extensions map[string]any `json:"-"`

	// cause is the underlying error (for error chaining).
	// Not serialized to JSON, but available via Unwrap().
	cause error
}

// NewProblemDetail creates a new RFC 9457 Problem Detail with a unique error ID.
func NewProblemDetail(status int, title string) *ProblemDetail {
	return &ProblemDetail{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Extensions: map[string]any{
			"error_id": generateErrorID(), // For ops correlation
		},
	}
}

// WithType sets the type URI and returns the ProblemDetail for chaining.
func (p *ProblemDetail) WithType(typeURI string) *ProblemDetail {
	p.Type = typeURI
	return p
}

// WithDetail sets the detail message and returns the ProblemDetail for chaining.
func (p *ProblemDetail) WithDetail(detail string) *ProblemDetail {
	p.Detail = detail
	return p
}

// WithInstance sets the instance URI and returns the ProblemDetail for chaining.
func (p *ProblemDetail) WithInstance(instance string) *ProblemDetail {
	p.Instance = instance
	return p
}

// WithCause sets the underlying error and returns the ProblemDetail for chaining.
func (p *ProblemDetail) WithCause(err error) *ProblemDetail {
	p.cause = err
	return p
}

// WithExtension adds an extension field and returns the ProblemDetail for chaining.
func (p *ProblemDetail) WithExtension(k string, v any) *ProblemDetail {
	if p.Extensions == nil {
		p.Extensions = make(map[string]any)
	}
	p.Extensions[k] = v
	return p
}

// WithExtensions adds multiple extension fields and returns the ProblemDetail for chaining.
func (p *ProblemDetail) WithExtensions(m map[string]any) *ProblemDetail {
	if p.Extensions == nil {
		p.Extensions = make(map[string]any, len(m))
	}
	maps.Copy(p.Extensions, m)
	return p
}

// Error implements the error interface.
func (p ProblemDetail) Error() string {
	if p.Detail != "" {
		return p.Detail
	}
	return p.Title
}

// Unwrap returns the underlying error for error chain compatibility.
// This allows using errors.Is() and errors.As() with ProblemDetail.
func (p *ProblemDetail) Unwrap() error {
	return p.cause
}

// MarshalJSON implements custom JSON marshaling to include extensions inline.
// Protects reserved field names from being overridden.
func (p ProblemDetail) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"type":   p.Type,
		"title":  p.Title,
		"status": p.Status,
	}

	if p.Detail != "" {
		m["detail"] = p.Detail
	}
	if p.Instance != "" {
		m["instance"] = p.Instance
	}

	// Add extensions inline, protecting reserved names
	for k, v := range p.Extensions {
		switch k {
		case "type", "title", "status", "detail", "instance":
			// Reserved - ignore to prevent override
		default:
			m[k] = v
		}
	}

	return json.Marshal(m)
}
