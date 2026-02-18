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
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

// Query binds URL query parameters to type T.
//
// Example:
//
//	params, err := binding.Query[ListParams](r.URL.Query())
//
// Errors:
//   - [ErrOutMustBePointer]: T is not a struct type
//   - [ErrMaxDepthExceeded]: struct nesting exceeds maximum depth
//   - [ErrSliceExceedsMaxLength]: slice length exceeds maximum
//   - [ErrMapExceedsMaxSize]: map size exceeds maximum
//   - [BindError]: field-level binding errors with detailed context
func Query[T any](values url.Values, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	defer cfg.finish()
	if err := bindFromSource(&result, NewQueryGetter(values), TagQuery, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// Path binds URL path parameters to type T.
//
// Example:
//
//	params, err := binding.Path[GetUserParams](pathParams)
func Path[T any](params map[string]string, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	defer cfg.finish()
	if err := bindFromSource(&result, NewPathGetter(params), TagPath, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// Form binds form data to type T.
//
// Example:
//
//	data, err := binding.Form[FormData](r.PostForm)
//
// Errors:
//   - [ErrOutMustBePointer]: T is not a struct type
//   - [ErrMaxDepthExceeded]: struct nesting exceeds maximum depth
//   - [ErrSliceExceedsMaxLength]: slice length exceeds maximum
//   - [ErrMapExceedsMaxSize]: map size exceeds maximum
//   - [BindError]: field-level binding errors with detailed context
func Form[T any](values url.Values, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	defer cfg.finish()
	if err := bindFromSource(&result, NewFormGetter(values), TagForm, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// Header binds HTTP headers to type T.
//
// Example:
//
//	headers, err := binding.Header[RequestHeaders](r.Header)
func Header[T any](h http.Header, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	defer cfg.finish()
	if err := bindFromSource(&result, NewHeaderGetter(h), TagHeader, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// Cookie binds cookies to type T.
//
// Example:
//
//	session, err := binding.Cookie[SessionData](r.Cookies())
//
// Errors:
//   - [ErrOutMustBePointer]: T is not a struct type
//   - [ErrMaxDepthExceeded]: struct nesting exceeds maximum depth
//   - [BindError]: field-level binding errors with detailed context
func Cookie[T any](cookies []*http.Cookie, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	defer cfg.finish()
	if err := bindFromSource(&result, NewCookieGetter(cookies), TagCookie, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// Multipart binds multipart form data including files to type T.
// This handles both uploaded files (*File, []*File fields) and regular form values.
//
// Example:
//
//	type UploadRequest struct {
//	    File     *File   `form:"avatar"`
//	    Files    []*File `form:"attachments"`
//	    Title    string  `form:"title"`
//	    Settings *Config `form:"settings"` // JSON auto-parsed
//	}
//
//	r.ParseMultipartForm(32 << 20) // 32 MB max memory
//	req, err := binding.Multipart[UploadRequest](r.MultipartForm)
//	if err != nil {
//	    return err
//	}
//	req.File.Save("./uploads/" + req.File.Name)
//
// Errors:
//   - [ErrOutMustBePointer]: T is not a struct type
//   - [ErrFileNotFound]: required file field not found
//   - [ErrMaxDepthExceeded]: struct nesting exceeds maximum depth
//   - [BindError]: field-level binding errors with detailed context
func Multipart[T any](form *multipart.Form, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	defer cfg.finish()
	if err := bindFromSource(&result, NewMultipartGetter(form), TagForm, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// Bind binds from one or more sources specified via From* options.
//
// Example:
//
//	req, err := binding.Bind[CreateOrderRequest](
//	    binding.FromPath(pathParams),
//	    binding.FromQuery(r.URL.Query()),
//	    binding.FromJSON(body),
//	)
//
// Errors:
//   - [ErrNoSourcesProvided]: no binding sources provided via From* options
//   - [ErrOutMustBePointer]: T is not a struct type
//   - [ErrMaxDepthExceeded]: struct nesting exceeds maximum depth
//   - [UnknownFieldError]: when [WithUnknownFields] is [UnknownError] and unknown fields are present
//   - [BindError]: field-level binding errors with detailed context
//   - [MultiError]: when [WithAllErrors] is used and multiple errors occur
func Bind[T any](opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	defer cfg.finish()
	if err := bindMultiSource(&result, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// QueryTo binds URL query parameters to out.
//
// Example:
//
//	var params ListParams
//	err := binding.QueryTo(r.URL.Query(), &params)
func QueryTo(values url.Values, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	defer cfg.finish()

	return bindFromSource(out, NewQueryGetter(values), TagQuery, cfg)
}

// PathTo binds URL path parameters to out.
//
// Example:
//
//	var params GetUserParams
//	err := binding.PathTo(pathParams, &params)
func PathTo(params map[string]string, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	defer cfg.finish()

	return bindFromSource(out, NewPathGetter(params), TagPath, cfg)
}

// FormTo binds form data to out.
//
// Example:
//
//	var data FormData
//	err := binding.FormTo(r.PostForm, &data)
func FormTo(values url.Values, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	defer cfg.finish()

	return bindFromSource(out, NewFormGetter(values), TagForm, cfg)
}

// HeaderTo binds HTTP headers to out.
//
// Example:
//
//	var headers RequestHeaders
//	err := binding.HeaderTo(r.Header, &headers)
func HeaderTo(h http.Header, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	defer cfg.finish()

	return bindFromSource(out, NewHeaderGetter(h), TagHeader, cfg)
}

// CookieTo binds cookies to out.
//
// Example:
//
//	var session SessionData
//	err := binding.CookieTo(r.Cookies(), &session)
func CookieTo(cookies []*http.Cookie, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	defer cfg.finish()

	return bindFromSource(out, NewCookieGetter(cookies), TagCookie, cfg)
}

// MultipartTo binds multipart form data including files to out.
// This handles both uploaded files (*File, []*File fields) and regular form values.
//
// Example:
//
//	type UploadRequest struct {
//	    File  *File  `form:"avatar"`
//	    Title string `form:"title"`
//	}
//
//	r.ParseMultipartForm(32 << 20)
//	var req UploadRequest
//	err := binding.MultipartTo(r.MultipartForm, &req)
func MultipartTo(form *multipart.Form, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	defer cfg.finish()

	return bindFromSource(out, NewMultipartGetter(form), TagForm, cfg)
}

// BindTo binds from one or more sources specified via From* options.
//
// Example:
//
//	var req CreateOrderRequest
//	err := binding.BindTo(&req,
//	    binding.FromPath(pathParams),
//	    binding.FromQuery(r.URL.Query()),
//	    binding.FromJSON(body),
//	)
func BindTo(out any, opts ...Option) error {
	cfg := applyOptions(opts)
	defer cfg.finish()

	return bindMultiSource(out, cfg)
}

// Raw binds values from a [ValueGetter] to out using the specified tag.
// This is the low-level binding function for custom sources.
//
// For built-in sources, prefer the type-safe functions: [Query], [Path], [Form], etc.
//
// Example:
//
//	customGetter := &MyCustomGetter{...}
//	err := binding.Raw(customGetter, "custom", &result)
//
// Errors:
//   - [ErrOutMustBePointer]: out is not a pointer to struct
//   - [ErrMaxDepthExceeded]: struct nesting exceeds maximum depth
//   - [BindError]: field-level binding errors with detailed context
func Raw(getter ValueGetter, tag string, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	defer cfg.finish()

	return bindFromSource(out, getter, tag, cfg)
}

// RawInto binds values from a [ValueGetter] to type T using the specified tag.
// This is the generic low-level binding function for custom sources.
//
// Example:
//
//	result, err := binding.RawInto[MyType](customGetter, "custom")
//
// Errors:
//   - [ErrOutMustBePointer]: T is not a struct type
//   - [ErrMaxDepthExceeded]: struct nesting exceeds maximum depth
//   - [BindError]: field-level binding errors with detailed context
func RawInto[T any](getter ValueGetter, tag string, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	defer cfg.finish()
	if err := bindFromSource(&result, getter, tag, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// applyOptions creates a new config with default values and applies the given options.
// It returns a configured config instance ready for use in binding operations.
func applyOptions(opts []Option) *config {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// bindFromSource binds values from a single source.
func bindFromSource(out any, getter ValueGetter, tag string, cfg *config) error {
	// Validate output is a pointer to struct
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Pointer {
		cfg.trackError()
		return ErrOutMustBePointer
	}

	if rv.IsNil() {
		cfg.trackError()
		return ErrOutPointerNil
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		cfg.trackError()
		return ErrOutMustBePointer
	}

	// Get cached struct info
	info := getStructInfo(elem.Type(), tag)

	// Bind fields with depth tracking
	return bindFieldsWithDepth(elem, getter, tag, info, cfg, 0)
}

// bindMultiSource binds from multiple sources configured via From* options.
// It handles JSON and XML sources specially, then processes other sources
// using the standard binding flow.
func bindMultiSource(out any, cfg *config) error {
	if len(cfg.sources) == 0 {
		cfg.trackError()
		return ErrNoSourcesProvided
	}

	// Validate output is a pointer to struct
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Pointer {
		cfg.trackError()
		return ErrOutMustBePointer
	}

	if rv.IsNil() {
		cfg.trackError()
		return ErrOutPointerNil
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		cfg.trackError()
		return ErrOutMustBePointer
	}

	var errs []error

	// Bind from each source
	for _, src := range cfg.sources {
		// Handle JSON sources specially
		if jsonSrc, ok := src.getter.(*jsonSourceGetter); ok {
			if err := bindJSONBytesInternal(out, jsonSrc.body, cfg); err != nil {
				if cfg.allErrors {
					errs = append(errs, err)
				} else {
					return err
				}
			}

			continue
		}

		if jsonReaderSrc, ok := src.getter.(*jsonReaderSourceGetter); ok {
			if err := bindJSONReaderInternal(out, jsonReaderSrc.reader, cfg); err != nil {
				if cfg.allErrors {
					errs = append(errs, err)
				} else {
					return err
				}
			}

			continue
		}

		// Handle XML sources specially
		if xmlSrc, ok := src.getter.(*xmlSourceGetter); ok {
			if err := bindXMLBytesInternal(out, xmlSrc.body, cfg); err != nil {
				if cfg.allErrors {
					errs = append(errs, err)
				} else {
					return err
				}
			}

			continue
		}

		if xmlReaderSrc, ok := src.getter.(*xmlReaderSourceGetter); ok {
			if err := bindXMLReaderInternal(out, xmlReaderSrc.reader, cfg); err != nil {
				if cfg.allErrors {
					errs = append(errs, err)
				} else {
					return err
				}
			}

			continue
		}

		// Check if struct has this tag
		if HasStructTag(elem.Type(), src.tag) {
			info := getStructInfo(elem.Type(), src.tag)
			if err := bindFieldsWithDepth(elem, src.getter, src.tag, info, cfg, 0); err != nil {
				if cfg.allErrors {
					errs = append(errs, err)
				} else {
					return err
				}
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// bindFieldsWithDepth binds all fields in a struct with depth enforcement.
// It handles maps, nested structs, slices, and single-value fields, applying defaults.
func bindFieldsWithDepth(elem reflect.Value, getter ValueGetter, tagName string,
	info *structInfo, cfg *config, depth int,
) error {
	// Enforce maximum nesting depth
	if depth > cfg.maxDepth {
		cfg.trackError()
		return fmt.Errorf("%w of %d", ErrMaxDepthExceeded, cfg.maxDepth)
	}

	// Cache event presence flags once per bind call
	evtFlags := cfg.eventFlags()

	// Track errors for allErrors mode
	var multiErr *MultiError
	if cfg.allErrors {
		multiErr = &MultiError{}
	}

	for _, field := range info.fields {
		// Get the field value by index path
		fieldValue := elem.FieldByIndex(field.index)
		if !fieldValue.CanSet() {
			continue // Skip unexported fields
		}

		// Handle file fields (*File or []*File)
		if isFileType(field.fieldType) {
			if err := setFileField(fieldValue, getter, field.tagName); err != nil {
				bindErr := &BindError{
					Field:  field.name,
					Source: sourceFromTag(tagName),
					Value:  "",
					Type:   fieldValue.Type(),
					Reason: err.Error(),
					Err:    err,
				}
				if cfg.allErrors {
					multiErr.Add(bindErr)

					continue
				}
				cfg.trackError()

				return bindErr
			}
			cfg.trackField(field.name, tagName, evtFlags)

			continue
		}

		// Handle map fields
		if field.isMap {
			if err := setMapField(fieldValue, getter, field.tagName, field.fieldType, cfg); err != nil {
				bindErr := &BindError{
					Field:  field.name,
					Source: sourceFromTag(tagName),
					Value:  "",
					Type:   fieldValue.Type(),
					Err:    err,
				}
				if cfg.allErrors {
					multiErr.Add(bindErr)

					continue
				}
				cfg.trackError()

				return bindErr
			}
			cfg.trackField(field.name, tagName, evtFlags)

			continue
		}

		// Handle nested struct fields (with incremented depth)
		if field.isStruct {
			if err := setNestedStructWithDepth(fieldValue, getter, field.tagName,
				tagName, cfg, depth+1); err != nil {
				bindErr := &BindError{
					Field:  field.name,
					Source: sourceFromTag(tagName),
					Value:  "",
					Type:   fieldValue.Type(),
					Err:    err,
				}
				if cfg.allErrors {
					multiErr.Add(bindErr)

					continue
				}
				cfg.trackError()

				return bindErr
			}
			cfg.trackField(field.name, tagName, evtFlags)

			continue
		}

		// Try primary name first
		value := getter.Get(field.tagName)
		hasValue := getter.Has(field.tagName)

		// Try aliases if primary is empty/missing
		if !hasValue && len(field.aliases) > 0 {
			for _, alias := range field.aliases {
				if getter.Has(alias) {
					value = getter.Get(alias)
					hasValue = true

					break
				}
			}
		}

		// Apply default value if no value provided and default is specified
		if !hasValue && field.defaultValue != "" {
			if applied := applyTypedDefault(elem, field); applied {
				cfg.trackField(field.name, tagName, evtFlags)

				continue
			}
			// Fallback: convert at runtime
			value = field.defaultValue
			hasValue = true
		}

		// Skip fields without values and no defaults
		if !hasValue {
			continue
		}

		// Handle slice fields
		if field.isSlice {
			values := getter.GetAll(field.tagName)
			if err := setSliceField(fieldValue, values, cfg); err != nil {
				bindErr := &BindError{
					Field:  field.name,
					Source: sourceFromTag(tagName),
					Value:  strings.Join(values, ","),
					Type:   fieldValue.Type(),
					Err:    err,
				}
				if cfg.allErrors {
					multiErr.Add(bindErr)
					continue
				}
				cfg.trackError()

				return bindErr
			}
			cfg.trackField(field.name, tagName, evtFlags)

			continue
		}

		// Handle single value fields (value already retrieved above)
		if err := setField(fieldValue, value, field.isPtr, cfg); err != nil {
			bindErr := &BindError{
				Field:  field.name,
				Source: sourceFromTag(tagName),
				Value:  value,
				Type:   fieldValue.Type(),
				Err:    err,
			}
			if cfg.allErrors {
				multiErr.Add(bindErr)
				continue
			}
			cfg.trackError()

			return bindErr
		}

		cfg.trackField(field.name, tagName, evtFlags)
	}

	if cfg.allErrors && multiErr.HasErrors() {
		return multiErr
	}

	return nil
}

// parseStructInfo parses struct fields and extracts binding information.
// It validates default values and computes typed defaults when possible.
// The result is cached by getStructInfo.
func parseStructInfo(t reflect.Type, tagName string) *structInfo {
	info := parseStructType(t, tagName, nil)

	// Validate tags and pre-compute expensive operations
	for i := range info.fields {
		field := &info.fields[i]

		// Validate default tag: ensure it's compatible with field type
		if field.defaultValue != "" {
			// Basic validation check (full validation happens at runtime)
			if field.isSlice || field.isMap {
				if err := invalidTagf("field %s: default tag not supported for slices/maps",
					field.name); err != nil {
					continue
				}
			}
		}
	}

	return info
}

// parseStructType recursively parses struct fields and extracts binding information.
// It handles embedded structs, pointer types, slices, maps, and nested structs.
// The indexPrefix parameter tracks the field index path for nested access.
// Called by [parseStructInfo] during struct metadata parsing.
func parseStructType(t reflect.Type, tagName string, indexPrefix []int) *structInfo {
	info := &structInfo{
		fields: make([]fieldInfo, 0, t.NumField()),
	}

	for i := range t.NumField() {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Build index path for nested access
		index := append(indexPrefix, i)

		// Handle embedded structs (anonymous fields)
		fieldType := field.Type
		kind := fieldType.Kind()

		// Check for pointer to struct (embedded)
		if kind == reflect.Pointer && fieldType.Elem().Kind() == reflect.Struct {
			fieldType = fieldType.Elem()
			kind = reflect.Struct
		}

		if field.Anonymous && kind == reflect.Struct {
			// Recursively parse embedded struct
			embeddedInfo := parseStructType(fieldType, tagName, index)
			info.fields = append(info.fields, embeddedInfo.fields...)

			continue
		}

		// Get tag value
		tag := field.Tag.Get(tagName)
		if tag == "" && tagName != TagJSON && tagName != TagForm {
			// For non-standard tags, skip if not present
			continue
		}

		// Handle json/form tags with options (e.g., "name,omitempty")
		if (tagName == TagJSON || tagName == TagForm) && tag == "-" {
			continue // Skip fields marked with "-"
		}

		isJSONOrForm := tagName == TagJSON || tagName == TagForm
		primaryName, aliases := parseTagWithAliases(tag, field.Name, isJSONOrForm)

		// Reset to original field type for further processing
		fieldType = field.Type
		kind = fieldType.Kind()

		// Handle pointer types
		isPtr := false
		if kind == reflect.Pointer {
			isPtr = true
			fieldType = fieldType.Elem()
			kind = fieldType.Kind()
		}

		// Handle slice types
		// Special case: net.IP is []byte but should be treated as a single value, not a slice
		isSlice := false
		elemKind := kind
		if kind == reflect.Slice && fieldType != ipType {
			isSlice = true
			elemType := fieldType.Elem()
			elemKind = elemType.Kind()

			// Handle []* types (slice of pointers)
			if elemKind == reflect.Pointer {
				elemKind = elemType.Elem().Kind()
			}
		}

		// Handle map types
		isMap := kind == reflect.Map

		// Handle nested struct types (non-embedded)
		isStruct := kind == reflect.Struct && fieldType != timeType && fieldType != urlType && fieldType != ipNetType && fieldType != regexpType

		// Get default value from tag
		defaultValue := field.Tag.Get("default")

		// Compute typed default value
		var typedDefault any
		hasTypedDefault := false
		if defaultValue != "" && !isSlice && !isMap {
			// Attempt to convert default value to typed form
			// Use default config for conversion (time layouts, etc.)
			defaultCfg := defaultConfig()
			if convertedVal, err := convertToType(defaultValue, field.Type, defaultCfg); err == nil {
				typedDefault = convertedVal.Interface()
				hasTypedDefault = true
			} else {
				// Use invalidTagf which panics in debug builds, returns error in prod
				//nolint:errcheck // Debug: panics; Prod: error intentionally ignored, fallback to runtime conversion
				invalidTagf("field %s: invalid default value %q for type %s: %v",
					field.Name, defaultValue, field.Type, err)
				// In debug builds: invalidTagf panics above, preventing startup with invalid config
				// In production builds: error is ignored, we continue without typed default
				// The default value will be converted at runtime when actually used
			}
		}

		// Add field info
		info.fields = append(info.fields, fieldInfo{
			index:           index,
			name:            field.Name,
			tagName:         primaryName,
			aliases:         aliases,
			kind:            kind,
			fieldType:       field.Type, // Store original field type (before unwrapping pointer)
			isPtr:           isPtr,
			isSlice:         isSlice,
			isMap:           isMap,
			isStruct:        isStruct,
			elemKind:        elemKind,
			defaultValue:    defaultValue,
			typedDefault:    typedDefault,
			hasTypedDefault: hasTypedDefault,
		})
	}

	return info
}

// HasStructTag checks if any field in the struct has the given tag.
// It recursively checks embedded structs to determine if the tag is present
// anywhere in the type hierarchy.
//
// This is useful for determining which binding sources should be used when
// binding from multiple sources with [Bind] or [BindTo].
//
// Parameters:
//   - t: Struct type to check
//   - tag: Tag name to search for (e.g., [TagJSON], [TagQuery])
//
// Returns true if any field (including in embedded structs) has the tag.
func HasStructTag(t reflect.Type, tag string) bool {
	return hasStructTagRecursive(t, tag, make(map[reflect.Type]bool))
}

// hasStructTagRecursive recursively checks for tags, avoiding infinite loops
// with embedded structs by tracking visited types.
func hasStructTagRecursive(t reflect.Type, tag string, visited map[reflect.Type]bool) bool {
	// Unwrap pointer
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	// Avoid infinite loops with circular embedded structs
	if visited[t] {
		return false
	}
	visited[t] = true

	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		// Check direct tag
		tagValue := field.Tag.Get(tag)
		if tagValue != "" && tagValue != "-" {
			return true
		}

		// Check embedded structs recursively
		if field.Anonymous {
			fieldType := field.Type
			// Handle pointer to struct
			if fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
			}
			// Recursively check embedded struct
			if fieldType.Kind() == reflect.Struct {
				if hasStructTagRecursive(fieldType, tag, visited) {
					return true
				}
			}
		}
	}

	return false
}

// parseTagWithAliases parses a struct tag value and extracts the primary name and aliases.
// For JSON/Form tags, it filters out modifiers like "omitempty".
// Returns the primary name (or fieldName if empty) and slice of aliases.
func parseTagWithAliases(tag, fieldName string, isJSONOrForm bool) (string, []string) {
	parts := strings.Split(tag, ",")
	primaryName := strings.TrimSpace(parts[0])

	var aliases []string
	for i := 1; i < len(parts); i++ {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		// Skip JSON-style modifiers for JSON/Form tags
		if isJSONOrForm && part == "omitempty" {
			continue
		}
		aliases = append(aliases, part)
	}

	// Use field name if tag is empty (for JSON/Form tags)
	if primaryName == "" && isJSONOrForm {
		primaryName = fieldName
	}

	return primaryName, aliases
}

// applyTypedDefault applies a pre-converted typed default value to the field.
// Returns true if the default was applied, false if fallback to runtime conversion is needed.
func applyTypedDefault(elem reflect.Value, field fieldInfo) bool {
	if !field.hasTypedDefault {
		return false
	}

	fv := elem.FieldByIndex(field.index)
	if !fv.CanSet() {
		return false
	}

	if field.isPtr {
		setPointerDefault(fv, field)
	} else {
		fv.Set(reflect.ValueOf(field.typedDefault))
	}

	return true
}

// setPointerDefault sets the default value for a pointer field.
func setPointerDefault(fv reflect.Value, field fieldInfo) {
	if fv.IsNil() {
		ptr := reflect.New(field.fieldType.Elem())
		ptr.Elem().Set(reflect.ValueOf(field.typedDefault))
		fv.Set(ptr)
	} else {
		fv.Elem().Set(reflect.ValueOf(field.typedDefault))
	}
}
