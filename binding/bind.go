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
	"reflect"
	"strings"
)

// Bind maps values from a ValueGetter to struct fields using the specified tag.
// It validates that out is a pointer to a struct, then binds matching values
// from the getter to struct fields based on struct tags.
//
// Bind supports nested structs, slices, maps, pointers, and custom types.
// It applies default values when specified in struct tags and validates enum
// values when present.
//
// Example:
//
//	type UserRequest struct {
//	    Name  string `query:"name"`
//	    Age   int    `query:"age"`
//	    Email string `query:"email"`
//	}
//
//	var req UserRequest
//	query := url.Values{"name": {"John"}, "age": {"30"}}
//	err := Bind(&req, NewQueryGetter(query), "query")
//
// Parameters:
//   - out: Pointer to struct that will receive bound values
//   - getter: ValueGetter that provides values (e.g., QueryGetter, FormGetter)
//   - tag: Struct tag name to use for field matching (e.g., "query", "form", "json")
//   - opts: Optional configuration options
//
// Returns an error if binding fails, including validation errors and type conversion errors.
func Bind(out any, getter ValueGetter, tag string, opts ...Option) error {
	options := applyOptions(opts)
	defer options.finish() // Always fire Done event

	// Validate output is a pointer to struct
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Ptr {
		options.trackError()
		return ErrOutMustBePointer
	}

	if rv.IsNil() {
		options.trackError()
		return ErrOutPointerNil
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		options.trackError()
		return ErrOutMustBePointer
	}

	// Get cached struct info
	info := getStructInfo(elem.Type(), tag)

	// Bind fields with depth tracking
	return bindFieldsWithDepth(elem, getter, tag, info, options, 0)
}

// bindFieldsWithDepth binds all fields in a struct with depth enforcement.
func bindFieldsWithDepth(elem reflect.Value, getter ValueGetter, tagName string,
	info *structInfo, opts *Options, depth int) error {

	// Enforce maximum nesting depth
	if depth > opts.MaxDepth {
		opts.trackError()
		return fmt.Errorf("%w of %d", ErrMaxDepthExceeded, opts.MaxDepth)
	}

	// Cache event presence flags once per bind call
	evtFlags := opts.eventFlags()

	for _, field := range info.fields {
		// Get the field value by index path
		fieldValue := elem.FieldByIndex(field.index)
		if !fieldValue.CanSet() {
			continue // Skip unexported fields
		}

		// Handle map fields
		if field.isMap {
			if err := setMapField(fieldValue, getter, field.tagName, field.fieldType, opts); err != nil {
				opts.trackError()
				return &BindError{
					Field: field.name,
					Tag:   tagName,
					Value: "",
					Type:  fieldValue.Type().String(),
					Err:   err,
				}
			}
			opts.trackField(field.name, tagName, evtFlags)
			continue
		}

		// Handle nested struct fields (with incremented depth)
		if field.isStruct {
			if err := setNestedStructWithDepth(fieldValue, getter, field.tagName,
				tagName, opts, depth+1); err != nil {
				opts.trackError()
				return &BindError{
					Field: field.name,
					Tag:   tagName,
					Value: "",
					Type:  fieldValue.Type().String(),
					Err:   err,
				}
			}
			opts.trackField(field.name, tagName, evtFlags)
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
			// Use pre-converted typed default if available
			if field.hasTypedDefault {
				fv := elem.FieldByIndex(field.index)
				if !fv.CanSet() {
					continue
				}
				if field.isPtr {
					if fv.IsNil() {
						ptr := reflect.New(field.fieldType.Elem())
						ptr.Elem().Set(reflect.ValueOf(field.typedDefault))
						fv.Set(ptr)
					} else {
						fv.Elem().Set(reflect.ValueOf(field.typedDefault))
					}
				} else {
					fv.Set(reflect.ValueOf(field.typedDefault))
				}
				opts.trackField(field.name, tagName, evtFlags)
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
			if err := setSliceField(fieldValue, values, opts); err != nil {
				opts.trackError()
				return &BindError{
					Field: field.name,
					Tag:   tagName,
					Value: strings.Join(values, ","),
					Type:  fieldValue.Type().String(),
					Err:   err,
				}
			}
			opts.trackField(field.name, tagName, evtFlags)
			continue
		}

		// Handle single value fields (value already retrieved above)

		// Enum validation
		if field.enumValues != "" {
			if err := validateEnum(value, field.enumValues); err != nil {
				opts.trackError()
				return &BindError{
					Field: field.name,
					Tag:   tagName,
					Value: value,
					Type:  fieldValue.Type().String(),
					Err:   err,
				}
			}
		}

		if err := setField(fieldValue, value, field.isPtr, opts); err != nil {
			opts.trackError()
			return &BindError{
				Field: field.name,
				Tag:   tagName,
				Value: value,
				Type:  fieldValue.Type().String(),
				Err:   err,
			}
		}

		opts.trackField(field.name, tagName, evtFlags)
	}

	return nil
}

// parseStructInfo parses struct fields and extracts binding information.
// It validates enum tags and default values, and computes typed defaults
// when possible. The result is cached by getStructInfo.
func parseStructInfo(t reflect.Type, tagName string) *structInfo {
	info := parseStructType(t, tagName, nil)

	// Validate tags and pre-compute expensive operations
	for i := range info.fields {
		field := &info.fields[i]

		// Validate enum tag: split once, check for duplicates
		if field.enumValues != "" {
			enums := strings.Split(field.enumValues, ",")
			seen := make(map[string]bool)
			for j, e := range enums {
				enums[j] = strings.TrimSpace(e)
				if enums[j] == "" {
					// Use invalidTagf which panics in debug builds, returns error in prod
					if err := invalidTagf("field %s: empty enum value in tag %q",
						field.name, field.enumValues); err != nil {
						// In non-debug builds, log and continue
						// (could integrate with logger here)
						continue
					}
				}
				if seen[enums[j]] {
					if err := invalidTagf("field %s: duplicate enum value %q",
						field.name, enums[j]); err != nil {
						continue
					}
				}
				seen[enums[j]] = true
			}
		}

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
		if kind == reflect.Ptr && fieldType.Elem().Kind() == reflect.Struct {
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
		primaryName := ""
		var aliases []string
		if tagName == TagJSON || tagName == TagForm {
			if tag == "-" {
				continue // Skip fields marked with "-"
			}
			// Split on comma for aliases (e.g., "user_id,id" -> primary="user_id", aliases=["id"])
			parts := strings.Split(tag, ",")
			primaryName = strings.TrimSpace(parts[0])
			// Collect aliases (skip json-style modifiers like "omitempty")
			for i := 1; i < len(parts); i++ {
				part := strings.TrimSpace(parts[i])
				if part != "" && part != "omitempty" && part != "required" {
					aliases = append(aliases, part)
				}
			}
			// Use field name if tag is empty
			if primaryName == "" {
				primaryName = field.Name
			}
		} else {
			// For other tags, split on comma for aliases
			parts := strings.Split(tag, ",")
			primaryName = strings.TrimSpace(parts[0])
			for i := 1; i < len(parts); i++ {
				part := strings.TrimSpace(parts[i])
				if part != "" {
					aliases = append(aliases, part)
				}
			}
		}

		// Reset to original field type for further processing
		fieldType = field.Type
		kind = fieldType.Kind()

		// Handle pointer types
		isPtr := false
		if kind == reflect.Ptr {
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
			if elemKind == reflect.Ptr {
				elemKind = elemType.Elem().Kind()
			}
		}

		// Handle map types
		isMap := kind == reflect.Map

		// Handle nested struct types (non-embedded)
		isStruct := kind == reflect.Struct && fieldType != timeType && fieldType != urlType && fieldType != ipNetType && fieldType != regexpType

		// Get enum validation values from tag
		enumValues := field.Tag.Get("enum")

		// Get default value from tag
		defaultValue := field.Tag.Get("default")

		// Compute typed default value
		var typedDefault any
		hasTypedDefault := false
		if defaultValue != "" && !isSlice && !isMap {
			// Attempt to convert default value to typed form
			// Use default options for conversion (time layouts, etc.)
			defaultOpts := defaultOptions()
			if convertedVal, err := convertToType(defaultValue, field.Type, defaultOpts); err == nil {
				typedDefault = convertedVal.Interface()
				hasTypedDefault = true
			} else {
				// Use invalidTagf which panics in debug builds, returns error in prod
				//nolint:errcheck // Error is intentionally ignored for graceful degradation
				_ = invalidTagf("field %s: invalid default value %q for type %s: %v",
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
			enumValues:      enumValues,
			defaultValue:    defaultValue,
			typedDefault:    typedDefault,
			hasTypedDefault: hasTypedDefault,
		})
	}

	return info
}

// BindInto binds values into a new instance of type T and returns it.
// It is a convenience wrapper around Bind that eliminates the need to
// create and pass a pointer manually.
//
// Example:
//
//	result, err := BindInto[UserRequest](getter, "query")
//
// Parameters:
//   - getter: ValueGetter that provides values
//   - tag: Struct tag name to use for field matching
//   - opts: Optional configuration options
//
// Returns the bound value of type T and an error if binding fails.
func BindInto[T any](getter ValueGetter, tag string, opts ...Option) (T, error) {
	var result T
	err := Bind(&result, getter, tag, opts...)
	return result, err
}

// SourceConfig maps a struct tag name to its ValueGetter.
// It is used by BindMulti to specify which source should be used for each tag type.
type SourceConfig struct {
	Tag    string      // Struct tag name (e.g., "query", "params", "header")
	Getter ValueGetter // ValueGetter for this tag type
}

// BindMulti binds values from multiple sources based on struct tags.
// It introspects the struct and only binds from sources where the corresponding
// tags are present in the struct fields.
//
// This is useful when a single struct contains fields with different tag types
// (e.g., query, params, header, cookie) and you want to bind them all at once.
//
// Example:
//
//	type Request struct {
//	    ID     string `path:"id"`
//	    Query  string `query:"q"`
//	    UserID string `header:"X-User-Id"`
//	}
//
//	sources := []SourceConfig{
//		{Tag: TagPath, Getter: NewPathGetter(params)},
//		{Tag: TagQuery, Getter: NewQueryGetter(query)},
//		{Tag: TagHeader, Getter: NewHeaderGetter(headers)},
//	}
//	var req Request
//	err := BindMulti(&req, sources)
//
// Parameters:
//   - out: Pointer to struct that will receive bound values
//   - sources: List of SourceConfig entries mapping tag names to ValueGetters
//   - opts: Optional configuration options
//
// Returns an error if any binding operation fails. Multiple errors are joined.
func BindMulti(out any, sources []SourceConfig, opts ...Option) error {
	// Validate output is a pointer to struct
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Ptr {
		return ErrOutMustBePointer
	}
	if rv.IsNil() {
		return ErrOutPointerNil
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return ErrOutMustBePointer
	}

	t := elem.Type()
	var errs []error

	// Bind from each source where tags exist
	for _, src := range sources {
		if HasStructTag(t, src.Tag) {
			if err := Bind(out, src.Getter, src.Tag, opts...); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", src.Tag, err))
			}
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// HasStructTag checks if any field in the struct has the given tag.
// It recursively checks embedded structs to determine if the tag is present
// anywhere in the type hierarchy.
//
// This is useful for determining which binding sources should be used when
// binding from multiple sources with BindMulti.
//
// Parameters:
//   - t: Struct type to check
//   - tag: Tag name to search for
//
// Returns true if any field (including in embedded structs) has the tag.
func HasStructTag(t reflect.Type, tag string) bool {
	return hasStructTagRecursive(t, tag, make(map[reflect.Type]bool))
}

// hasStructTagRecursive recursively checks for tags, avoiding infinite loops
// with embedded structs by tracking visited types.
func hasStructTagRecursive(t reflect.Type, tag string, visited map[reflect.Type]bool) bool {
	// Avoid infinite loops with circular embedded structs
	if visited[t] {
		return false
	}
	visited[t] = true

	for i := 0; i < t.NumField(); i++ {
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
			if fieldType.Kind() == reflect.Ptr {
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
