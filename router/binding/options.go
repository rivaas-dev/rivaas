package binding

import (
	"net/http"
	"reflect"
	"strings"
	"time"
)

// UnknownFieldPolicy defines how to handle unknown fields during JSON decoding.
type UnknownFieldPolicy int

const (
	// UnknownIgnore silently ignores unknown JSON fields (fastest, default).
	UnknownIgnore UnknownFieldPolicy = iota

	// UnknownWarn emits warnings via Events.UnknownField but continues binding.
	// ⚠️  PERFORMANCE: Uses 2-pass parsing (unmarshal + decode). Approximately
	// 1.5-2x slower than UnknownIgnore. Recommended for development/testing only.
	UnknownWarn

	// UnknownError returns an error on the first unknown field (strict mode).
	// Uses json.Decoder.DisallowUnknownFields with negligible overhead.
	UnknownError
)

// SliceParseMode defines how slice values are parsed from query/form data.
type SliceParseMode int

const (
	SliceRepeat SliceParseMode = iota // ?tags=a&tags=b&tags=c (default)
	SliceCSV                          // ?tags=a,b,c
)

// Security and resilience limits
const (
	// DefaultMaxDepth is the default maximum nesting depth for structs/maps.
	// Prevents stack overflow from malicious deeply-nested payloads.
	DefaultMaxDepth = 32

	// DefaultMaxMapSize is the default maximum number of map entries per field.
	// Prevents resource exhaustion from large map bindings.
	DefaultMaxMapSize = 1000

	// DefaultMaxSliceLen is the default maximum number of slice elements per field.
	// Prevents memory exhaustion from large slice bindings.
	DefaultMaxSliceLen = 10_000

	// DefaultMaxBodySize is the default maximum request body size (10 MiB).
	// This is enforced at the router layer, not in binding.
	DefaultMaxBodySize = 10 << 20
)

// TypeConverter converts a string value to a custom type.
// Registered converters are checked before built-in type handling.
type TypeConverter func(string) (any, error)

// KeyNormalizer transforms keys before lookup (e.g., case-folding, canonicalization).
type KeyNormalizer func(string) string

// Events provides hooks for observability without coupling.
type Events struct {
	// FieldBound is called after successfully binding a field.
	// name: struct field name, fromTag: source tag (query, json, etc.)
	FieldBound func(name string, fromTag string)

	// UnknownField is called when an unknown field is encountered.
	// Only triggered when UnknownFieldPolicy is UnknownWarn or UnknownError.
	// path: dot-separated field path (e.g., "user.address.unknown")
	UnknownField func(path string)

	// Done is called at the end of binding with statistics.
	// Always called, even on error (use defer).
	Done func(stats Stats)
}

// Stats tracks binding operation metrics.
type Stats struct {
	FieldsProcessed   int           // Total fields attempted
	FieldsBound       int           // Successfully bound fields
	ErrorsEncountered int           // Errors hit during binding
	Duration          time.Duration // Total binding time (if tracked externally)
}

// Options configures binding behavior.
//
// Options are applied per-call via functional options. It is safe to reuse
// Option functions across goroutines, but Options instances should not be
// reused (applyOptions creates a fresh instance each time).
type Options struct {
	// Existing options
	TimeLayouts         []string // Custom time layouts (default: RFC3339, etc.)
	CaseInsensitiveKeys bool     // For query/form params (not implemented yet, reserved for future)
	MaxDepth            int      // Max nesting depth for structs
	ErrorsAsMulti       bool     // Return MultiError instead of first error (not implemented yet, reserved for future)

	// New options
	UnknownFields  UnknownFieldPolicy             // How to handle unknown JSON fields
	TypeConverters map[reflect.Type]TypeConverter // Custom type converters
	Events         Events                         // Observability hooks
	SliceMode      SliceParseMode                 // How to parse slice values
	IntBaseAuto    bool                           // Auto-detect integer bases (0x, 0, 0b)
	JSONUseNumber  bool                           // Use json.Number instead of float64
	KeyNormalizer  KeyNormalizer                  // Custom key normalization

	// Internal limits (not exported; use WithMaxXxx options to configure)
	maxMapSize  int // Maximum map entries per field
	maxSliceLen int // Maximum slice elements per field

	// Internal state (not set by users)
	stats Stats // Accumulated statistics during binding
}

// Option configures binding behavior.
type Option func(*Options)

// WithTimeLayouts sets custom time parsing layouts.
// Default layouts are tried first, then custom ones.
func WithTimeLayouts(layouts ...string) Option {
	return func(o *Options) {
		o.TimeLayouts = layouts
	}
}

// WithCaseInsensitiveKeys enables case-insensitive key matching for query/form params.
// Reserved for future implementation.
func WithCaseInsensitiveKeys(v bool) Option {
	return func(o *Options) {
		o.CaseInsensitiveKeys = v
	}
}

// WithMaxDepth sets the maximum nesting depth for structs.
func WithMaxDepth(depth int) Option {
	return func(o *Options) {
		o.MaxDepth = depth
	}
}

// WithUnknownFieldPolicy sets how to handle unknown JSON fields.
func WithUnknownFieldPolicy(policy UnknownFieldPolicy) Option {
	return func(o *Options) {
		o.UnknownFields = policy
	}
}

// WithTypeConverter registers a custom converter for a type.
// The converter is called before built-in type handling.
// Works transparently for both T and *T (pointer normalization).
func WithTypeConverter(targetType reflect.Type, converter TypeConverter) Option {
	return func(o *Options) {
		if o.TypeConverters == nil {
			o.TypeConverters = make(map[reflect.Type]TypeConverter)
		}
		o.TypeConverters[targetType] = converter
	}
}

// WithTypedConverter provides type-safe converter registration.
// This is safer than WithTypeConverter as it enforces type correctness at compile time.
//
// Example:
//
//	WithTypedConverter(func(s string) (uuid.UUID, error) {
//		return uuid.Parse(s)
//	})
func WithTypedConverter[T any](fn func(string) (T, error)) Option {
	return func(o *Options) {
		targetType := reflect.TypeFor[T]()

		if o.TypeConverters == nil {
			o.TypeConverters = make(map[reflect.Type]TypeConverter)
		}

		// Wrap the typed function into TypeConverter
		o.TypeConverters[targetType] = func(s string) (any, error) {
			return fn(s)
		}
	}
}

// WithEvents sets observability hooks.
func WithEvents(events Events) Option {
	return func(o *Options) {
		o.Events = events
	}
}

// WithSliceParseMode sets how to parse slice values.
func WithSliceParseMode(mode SliceParseMode) Option {
	return func(o *Options) {
		o.SliceMode = mode
	}
}

// WithIntBaseAuto enables auto-detection of integer bases (0x, 0, 0b prefixes).
func WithIntBaseAuto(enabled bool) Option {
	return func(o *Options) {
		o.IntBaseAuto = enabled
	}
}

// WithJSONUseNumber makes the JSON decoder use json.Number (avoids float round-trips).
func WithJSONUseNumber(enabled bool) Option {
	return func(o *Options) {
		o.JSONUseNumber = enabled
	}
}

// WithKeyNormalizer sets a custom key normalization function.
func WithKeyNormalizer(normalizer KeyNormalizer) Option {
	return func(o *Options) {
		o.KeyNormalizer = normalizer
	}
}

// WithMaxMapSize sets the maximum number of map entries per field.
// Default is DefaultMaxMapSize (1000). Set to 0 to disable limit.
func WithMaxMapSize(n int) Option {
	return func(o *Options) {
		o.maxMapSize = n
	}
}

// WithMaxSliceLen sets the maximum number of slice elements per field.
// Default is DefaultMaxSliceLen (10,000). Set to 0 to disable limit.
func WithMaxSliceLen(n int) Option {
	return func(o *Options) {
		o.maxSliceLen = n
	}
}

// Common normalizers
var (
	// CanonicalMIME normalizes HTTP header keys (Content-Type -> Content-Type)
	CanonicalMIME KeyNormalizer = http.CanonicalHeaderKey

	// LowerCase converts keys to lowercase (case-insensitive matching)
	LowerCase KeyNormalizer = strings.ToLower
)

// defaultOptions returns default binding options.
func defaultOptions() *Options {
	return &Options{
		TimeLayouts: []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02",
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
			"2006-01-02T15:04:05Z07:00",
		},
		MaxDepth:      DefaultMaxDepth,    // Safe default: 32
		UnknownFields: UnknownIgnore,      // Safe default
		SliceMode:     SliceRepeat,        // Default: repeated keys
		maxMapSize:    DefaultMaxMapSize,  // Safe default: 1000
		maxSliceLen:   DefaultMaxSliceLen, // Safe default: 10,000
	}
}

// applyOptions applies options to default options.
func applyOptions(opts []Option) *Options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// eventFlags caches event presence to avoid repeated nil checks.
type eventFlags struct {
	hasFieldBound   bool
	hasUnknownField bool
	hasDone         bool
}

// eventFlags computes event presence flags once.
func (o *Options) eventFlags() eventFlags {
	return eventFlags{
		hasFieldBound:   o.Events.FieldBound != nil,
		hasUnknownField: o.Events.UnknownField != nil,
		hasDone:         o.Events.Done != nil,
	}
}

// trackFieldFast records a field with cached event flags (micro-optimization).
func (o *Options) trackFieldFast(fieldName, sourceTag string, flags eventFlags) {
	o.stats.FieldsProcessed++
	o.stats.FieldsBound++
	if flags.hasFieldBound {
		o.Events.FieldBound(fieldName, sourceTag)
	}
}

// trackError records an error during binding.
func (o *Options) trackError() {
	o.stats.ErrorsEncountered++
}

// finish emits the Done event with final statistics.
// Always called via defer in Bind(), even on error.
func (o *Options) finish() {
	if o.Events.Done != nil {
		o.Events.Done(o.stats)
	}
}
