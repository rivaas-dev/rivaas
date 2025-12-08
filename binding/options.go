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

package binding

import (
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
)

// UnknownFieldPolicy defines how to handle unknown fields during JSON decoding.
type UnknownFieldPolicy int

const (
	// UnknownIgnore silently ignores unknown JSON fields.
	// This is the default policy.
	UnknownIgnore UnknownFieldPolicy = iota

	// UnknownWarn emits warnings via Events.UnknownField but continues binding.
	// It uses two-pass parsing to detect unknown fields at all nesting levels.
	// Recommended for development and testing environments.
	UnknownWarn

	// UnknownError returns an error on the first unknown field.
	// It uses json.Decoder.DisallowUnknownFields for strict validation.
	UnknownError
)

// SliceParseMode defines how slice values are parsed from query/form data.
type SliceParseMode int

const (
	SliceRepeat SliceParseMode = iota // ?tags=a&tags=b&tags=c (default)
	SliceCSV                          // ?tags=a,b,c
)

// Security and resilience limits for binding operations.
const (
	// DefaultMaxDepth is the default maximum nesting depth for structs and maps.
	// It prevents stack overflow from malicious deeply-nested payloads.
	DefaultMaxDepth = 32

	// DefaultMaxMapSize is the default maximum number of map entries per field.
	// It prevents resource exhaustion from large map bindings.
	DefaultMaxMapSize = 1000

	// DefaultMaxSliceLen is the default maximum number of slice elements per field.
	// It prevents memory exhaustion from large slice bindings.
	DefaultMaxSliceLen = 10_000

	// DefaultMaxBodySize is the default maximum request body size (10 MiB).
	// This limit is enforced at the router layer, not in the binding package.
	DefaultMaxBodySize = 10 << 20
)

// TypeConverter converts a string value to a custom type.
// Registered converters are checked before built-in type handling.
// If a converter returns an error, binding fails for that field.
type TypeConverter func(string) (any, error)

// KeyNormalizer transforms keys before lookup.
// Common uses include case-folding and canonicalization.
type KeyNormalizer func(string) string

// Validator validates a struct after binding.
type Validator interface {
	Validate(v any) error
}

// Events provides hooks for observability without coupling.
type Events struct {
	// FieldBound is called after successfully binding a field.
	// name: struct field name, fromTag: source tag (query, json, etc.)
	FieldBound func(name, fromTag string)

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

// sourceEntry represents a binding source with its getter and tag.
type sourceEntry struct {
	getter ValueGetter
	tag    string
}

// config holds internal binding configuration.
type config struct {
	// Parsing options
	timeLayouts []string       // Custom time layouts (default: RFC3339, etc.)
	sliceMode   SliceParseMode // How to parse slice values
	intBaseAuto bool           // Auto-detect integer bases (0x, 0, 0b)

	// Limits
	maxDepth    int // Max nesting depth for structs
	maxMapSize  int // Maximum map entries per field
	maxSliceLen int // Maximum slice elements per field

	// JSON options
	unknownFields UnknownFieldPolicy // How to handle unknown JSON fields
	jsonUseNumber bool               // Use json.Number instead of float64

	// XML options
	xmlStrict bool // Use strict XML parsing mode

	// Type conversion
	typeConverters map[reflect.Type]TypeConverter // Custom type converters

	// Validation
	required  bool      // Check required tags
	validator Validator // External validator

	// Error handling
	allErrors bool // Collect all errors instead of returning on first

	// Observability
	events Events // Event hooks

	// Key normalization
	keyNormalizer KeyNormalizer // Custom key normalization

	// Sources for multi-source binding (populated by From* options)
	sources []sourceEntry

	// Internal state (not set by users)
	stats Stats // Accumulated statistics during binding
}

// Option configures binding behavior.
type Option func(*config)

// FromQuery specifies query parameters as a binding source for [Bind] or [BindTo].
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromQuery(r.URL.Query()),
//	    binding.FromPath(pathParams),
//	)
func FromQuery(values url.Values) Option {
	return func(c *config) {
		c.sources = append(c.sources, sourceEntry{
			getter: NewQueryGetter(values),
			tag:    TagQuery,
		})
	}
}

// FromPath specifies path parameters as a binding source.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromPath(pathParams),
//	)
func FromPath(params map[string]string) Option {
	return func(c *config) {
		c.sources = append(c.sources, sourceEntry{
			getter: NewPathGetter(params),
			tag:    TagPath,
		})
	}
}

// FromForm specifies form data as a binding source for [Bind] or [BindTo].
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromForm(r.PostForm),
//	)
func FromForm(values url.Values) Option {
	return func(c *config) {
		c.sources = append(c.sources, sourceEntry{
			getter: NewFormGetter(values),
			tag:    TagForm,
		})
	}
}

// FromHeader specifies HTTP headers as a binding source.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromHeader(r.Header),
//	)
func FromHeader(h http.Header) Option {
	return func(c *config) {
		c.sources = append(c.sources, sourceEntry{
			getter: NewHeaderGetter(h),
			tag:    TagHeader,
		})
	}
}

// FromCookie specifies cookies as a binding source for [Bind] or [BindTo].
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromCookie(r.Cookies()),
//	)
func FromCookie(cookies []*http.Cookie) Option {
	return func(c *config) {
		c.sources = append(c.sources, sourceEntry{
			getter: NewCookieGetter(cookies),
			tag:    TagCookie,
		})
	}
}

// FromJSON specifies JSON body as a binding source for [Bind] or [BindTo].
// Note: JSON binding is handled separately from other sources.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromQuery(r.URL.Query()),
//	    binding.FromJSON(body),
//	)
func FromJSON(body []byte) Option {
	return func(c *config) {
		c.sources = append(c.sources, sourceEntry{
			getter: &jsonSourceGetter{body: body},
			tag:    TagJSON,
		})
	}
}

// FromJSONReader specifies JSON from io.Reader as a binding source.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromJSONReader(r.Body),
//	)
func FromJSONReader(r io.Reader) Option {
	return func(c *config) {
		c.sources = append(c.sources, sourceEntry{
			getter: &jsonReaderSourceGetter{reader: r},
			tag:    TagJSON,
		})
	}
}

// FromXML specifies XML body as a binding source.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromQuery(r.URL.Query()),
//	    binding.FromXML(body),
//	)
func FromXML(body []byte) Option {
	return func(c *config) {
		c.sources = append(c.sources, sourceEntry{
			getter: &xmlSourceGetter{body: body},
			tag:    TagXML,
		})
	}
}

// FromXMLReader specifies XML from io.Reader as a binding source.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromXMLReader(r.Body),
//	)
func FromXMLReader(r io.Reader) Option {
	return func(c *config) {
		c.sources = append(c.sources, sourceEntry{
			getter: &xmlReaderSourceGetter{reader: r},
			tag:    TagXML,
		})
	}
}

// FromGetter specifies a custom ValueGetter as a binding source.
// Use this for custom binding sources not covered by the built-in options.
//
// Example:
//
//	customGetter := &MyCustomGetter{...}
//	req, err := binding.Bind[Request](
//	    binding.FromGetter(customGetter, "custom"),
//	)
func FromGetter(getter ValueGetter, tag string) Option {
	return func(c *config) {
		c.sources = append(c.sources, sourceEntry{
			getter: getter,
			tag:    tag,
		})
	}
}

// WithTimeLayouts sets custom time parsing layouts.
// Default layouts are tried first, then custom layouts are attempted.
// Layouts use Go's time format reference time: Mon Jan 2 15:04:05 MST 2006.
//
// Example:
//
//	binding.Query[T](values,
//	    binding.WithTimeLayouts("2006-01-02", "01/02/2006"),
//	)
func WithTimeLayouts(layouts ...string) Option {
	return func(c *config) {
		c.timeLayouts = layouts
	}
}

// WithSliceMode sets how slice values are parsed from query/form data.
// SliceRepeat (default) expects repeated keys: ?tags=a&tags=b&tags=c
// SliceCSV expects comma-separated values: ?tags=a,b,c
//
// Example:
//
//	binding.Query[T](values, binding.WithSliceMode(binding.SliceCSV))
func WithSliceMode(mode SliceParseMode) Option {
	return func(c *config) {
		c.sliceMode = mode
	}
}

// WithIntBaseAuto enables auto-detection of integer bases from prefixes.
// When enabled, recognizes 0x (hex), 0 (octal), and 0b (binary) prefixes.
//
// Example:
//
//	binding.Query[T](values, binding.WithIntBaseAuto())
func WithIntBaseAuto() Option {
	return func(c *config) {
		c.intBaseAuto = true
	}
}

// WithMaxDepth sets the maximum nesting depth for structs and maps.
// When exceeded, binding returns [ErrMaxDepthExceeded].
// The default is [DefaultMaxDepth] (32).
//
// Example:
//
//	binding.JSON[T](body, binding.WithMaxDepth(16))
func WithMaxDepth(depth int) Option {
	return func(c *config) {
		c.maxDepth = depth
	}
}

// WithMaxSliceLen sets the maximum number of slice elements per field.
// When exceeded, binding returns ErrSliceExceedsMaxLength.
// The default is DefaultMaxSliceLen (10,000). Set to 0 to disable the limit.
//
// Example:
//
//	binding.Query[T](values, binding.WithMaxSliceLen(1000))
func WithMaxSliceLen(n int) Option {
	return func(c *config) {
		c.maxSliceLen = n
	}
}

// WithMaxMapSize sets the maximum number of map entries per field.
// When exceeded, binding returns ErrMapExceedsMaxSize.
// The default is DefaultMaxMapSize (1000). Set to 0 to disable the limit.
//
// Example:
//
//	binding.Query[T](values, binding.WithMaxMapSize(500))
func WithMaxMapSize(n int) Option {
	return func(c *config) {
		c.maxMapSize = n
	}
}

// WithUnknownFields sets how to handle unknown JSON fields.
// See [UnknownFieldPolicy] for available policies.
//
// Example:
//
//	binding.JSON[T](body, binding.WithUnknownFields(binding.UnknownError))
func WithUnknownFields(policy UnknownFieldPolicy) Option {
	return func(c *config) {
		c.unknownFields = policy
	}
}

// WithJSONUseNumber configures the JSON decoder to use json.Number instead of float64.
// This preserves numeric precision for large integers that would otherwise be
// represented as floats.
//
// Example:
//
//	binding.JSON[T](body, binding.WithJSONUseNumber())
func WithJSONUseNumber() Option {
	return func(c *config) {
		c.jsonUseNumber = true
	}
}

// WithXMLStrict enables strict XML parsing mode.
// When enabled, the XML decoder will be more strict about element/attribute names.
//
// Example:
//
//	binding.XML[T](body, binding.WithXMLStrict())
func WithXMLStrict() Option {
	return func(c *config) {
		c.xmlStrict = true
	}
}

// WithConverter registers a custom type converter.
// Type-safe registration using generics.
//
// Example:
//
//	binding.MustNew(
//	    binding.WithConverter[uuid.UUID](uuid.Parse),
//	    binding.WithConverter[decimal.Decimal](decimal.NewFromString),
//	)
func WithConverter[T any](fn func(string) (T, error)) Option {
	return func(c *config) {
		targetType := reflect.TypeFor[T]()
		if c.typeConverters == nil {
			c.typeConverters = make(map[reflect.Type]TypeConverter)
		}
		c.typeConverters[targetType] = func(s string) (any, error) {
			return fn(s)
		}
	}
}

// WithTypeConverter registers a custom converter using reflect.Type.
// Use WithConverter[T] for type-safe registration when possible.
//
// Example:
//
//	binding.MustNew(
//	    binding.WithTypeConverter(
//	        reflect.TypeFor[uuid.UUID](),
//	        func(s string) (any, error) { return uuid.Parse(s) },
//	    ),
//	)
func WithTypeConverter(targetType reflect.Type, converter TypeConverter) Option {
	return func(c *config) {
		if c.typeConverters == nil {
			c.typeConverters = make(map[reflect.Type]TypeConverter)
		}
		c.typeConverters[targetType] = converter
	}
}

// WithRequired enables checking of `required` struct tags.
// When enabled, missing required fields return ErrRequiredField.
//
// Example:
//
//	type User struct {
//	    Name  string `json:"name" required:"true"`
//	    Email string `json:"email" required:"true"`
//	}
//
//	user, err := binding.JSON[User](body, binding.WithRequired())
func WithRequired() Option {
	return func(c *config) {
		c.required = true
	}
}

// WithValidator integrates external validation via a [Validator] implementation.
// The validator is called after successful binding.
//
// Example:
//
//	binding.MustNew(binding.WithValidator(myValidator))
func WithValidator(v Validator) Option {
	return func(c *config) {
		c.validator = v
	}
}

// WithAllErrors collects all binding errors instead of returning on first.
// When enabled, returns *MultiError containing all field errors.
//
// Example:
//
//	user, err := binding.JSON[User](body, binding.WithAllErrors())
//	if err != nil {
//	    var multi *binding.MultiError
//	    if errors.As(err, &multi) {
//	        for _, e := range multi.Errors {
//	            // Handle each error
//	        }
//	    }
//	}
func WithAllErrors() Option {
	return func(c *config) {
		c.allErrors = true
	}
}

// WithEvents sets observability hooks.
//
// Example:
//
//	binding.MustNew(binding.WithEvents(binding.Events{
//	    FieldBound: func(name, tag string) {
//	        log.Printf("Bound field %s from %s", name, tag)
//	    },
//	    Done: func(stats binding.Stats) {
//	        log.Printf("Binding complete: %d fields", stats.FieldsBound)
//	    },
//	}))
func WithEvents(events Events) Option {
	return func(c *config) {
		c.events = events
	}
}

// WithKeyNormalizer sets a custom key normalization function.
//
// Example:
//
//	binding.Header[T](h, binding.WithKeyNormalizer(binding.CanonicalMIME))
func WithKeyNormalizer(normalizer KeyNormalizer) Option {
	return func(c *config) {
		c.keyNormalizer = normalizer
	}
}

// Common normalizers
var (
	// CanonicalMIME normalizes HTTP header keys (Content-Type -> Content-Type)
	CanonicalMIME KeyNormalizer = http.CanonicalHeaderKey

	// LowerCase converts keys to lowercase (case-insensitive matching)
	LowerCase KeyNormalizer = strings.ToLower
)

// defaultConfig returns a new config with default binding configuration.
// Default values include:
//   - maxDepth: [DefaultMaxDepth] (32)
//   - maxMapSize: [DefaultMaxMapSize] (1,000)
//   - maxSliceLen: [DefaultMaxSliceLen] (10,000)
//   - unknownFields: [UnknownIgnore]
//   - sliceMode: [SliceRepeat]
//   - timeLayouts: RFC3339, RFC3339Nano, and common date formats
func defaultConfig() *config {
	return &config{
		timeLayouts: []string{
			time.RFC3339,
			time.RFC3339Nano,
			time.DateOnly,
			time.DateTime,
			"2006-01-02T15:04:05",
		},
		maxDepth:      DefaultMaxDepth,
		unknownFields: UnknownIgnore,
		sliceMode:     SliceRepeat,
		maxMapSize:    DefaultMaxMapSize,
		maxSliceLen:   DefaultMaxSliceLen,
	}
}

// validate validates the configuration.
func (c *config) validate() error {
	if c.maxDepth < 0 {
		return fmt.Errorf("binding: maxDepth must be non-negative, got %d", c.maxDepth)
	}
	if c.maxMapSize < 0 {
		return fmt.Errorf("binding: maxMapSize must be non-negative, got %d", c.maxMapSize)
	}
	if c.maxSliceLen < 0 {
		return fmt.Errorf("binding: maxSliceLen must be non-negative, got %d", c.maxSliceLen)
	}

	return nil
}

// clone creates a copy of the config for per-call modification.
func (c *config) clone() *config {
	clone := *c
	// Deep copy sources slice
	if c.sources != nil {
		clone.sources = make([]sourceEntry, len(c.sources))
		copy(clone.sources, c.sources)
	}
	// Deep copy type converters map
	if c.typeConverters != nil {
		clone.typeConverters = make(map[reflect.Type]TypeConverter, len(c.typeConverters))
		maps.Copy(clone.typeConverters, c.typeConverters)
	}

	return &clone
}

// eventFlags stores event presence flags.
type eventFlags struct {
	hasFieldBound   bool
	hasUnknownField bool
	hasDone         bool
}

// eventFlags computes event presence flags once.
func (c *config) eventFlags() eventFlags {
	return eventFlags{
		hasFieldBound:   c.events.FieldBound != nil,
		hasUnknownField: c.events.UnknownField != nil,
		hasDone:         c.events.Done != nil,
	}
}

// trackField records a field that was successfully bound, using event flags
// to check for event handlers.
func (c *config) trackField(fieldName, sourceTag string, flags eventFlags) {
	c.stats.FieldsProcessed++
	c.stats.FieldsBound++
	if flags.hasFieldBound {
		c.events.FieldBound(fieldName, sourceTag)
	}
}

// trackError records an error during binding.
func (c *config) trackError() {
	c.stats.ErrorsEncountered++
}

// finish emits the Done event with final statistics.
// Always called via defer in binding functions, even on error.
func (c *config) finish() {
	if c.events.Done != nil {
		c.events.Done(c.stats)
	}
}

// jsonSourceGetter is a marker type for JSON body source.
type jsonSourceGetter struct {
	body []byte
}

func (j *jsonSourceGetter) Get(key string) string      { return "" }
func (j *jsonSourceGetter) GetAll(key string) []string { return nil }
func (j *jsonSourceGetter) Has(key string) bool        { return false }

// jsonReaderSourceGetter is a marker type for JSON reader source.
type jsonReaderSourceGetter struct {
	reader io.Reader
}

func (j *jsonReaderSourceGetter) Get(key string) string      { return "" }
func (j *jsonReaderSourceGetter) GetAll(key string) []string { return nil }
func (j *jsonReaderSourceGetter) Has(key string) bool        { return false }

// xmlSourceGetter is a marker type for XML body source.
type xmlSourceGetter struct {
	body []byte
}

func (x *xmlSourceGetter) Get(key string) string      { return "" }
func (x *xmlSourceGetter) GetAll(key string) []string { return nil }
func (x *xmlSourceGetter) Has(key string) bool        { return false }

// xmlReaderSourceGetter is a marker type for XML reader source.
type xmlReaderSourceGetter struct {
	reader io.Reader
}

func (x *xmlReaderSourceGetter) Get(key string) string      { return "" }
func (x *xmlReaderSourceGetter) GetAll(key string) []string { return nil }
func (x *xmlReaderSourceGetter) Has(key string) bool        { return false }
