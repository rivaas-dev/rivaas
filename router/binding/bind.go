package binding

import (
	"fmt"
	"reflect"
	"strings"
)

// Bind maps values from getter to struct fields using reflection.
// This is the core binding function that handles all field types.
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
		return fmt.Errorf("exceeded maximum nesting depth of %d", opts.MaxDepth)
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
			opts.trackFieldFast(field.name, tagName, evtFlags)
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
			opts.trackFieldFast(field.name, tagName, evtFlags)
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
			// Fast path: use pre-converted typed default
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
				opts.trackFieldFast(field.name, tagName, evtFlags)
				continue
			}
			// Fallback: convert at runtime (shouldn't happen after warmup)
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
			opts.trackFieldFast(field.name, tagName, evtFlags)
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

		opts.trackFieldFast(field.name, tagName, evtFlags)
	}

	return nil
}

// parseStructInfo parses struct fields and extracts binding information.
// This is called by getStructInfo and cached.
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
			// Quick sanity check (full validation happens at runtime)
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

		// Pre-compute typed default value
		var typedDefault interface{}
		hasTypedDefault := false
		if defaultValue != "" && !isSlice && !isMap {
			// Attempt to pre-convert default value to typed form
			// Use default options for conversion (time layouts, etc.)
			defaultOpts := defaultOptions()
			if convertedVal, err := convertToType(defaultValue, field.Type, defaultOpts); err == nil {
				typedDefault = convertedVal.Interface()
				hasTypedDefault = true
			} else {
				// Fail fast: invalid default at startup
				// Use invalidTagf which panics in debug builds, returns error in prod
				if err := invalidTagf("field %s: invalid default value %q for type %s: %v",
					field.Name, defaultValue, field.Type, err); err != nil {
					// In non-debug builds, log and continue (will convert at runtime)
					// In debug builds, this panics above
				}
				// Continue without typed default (will convert at runtime)
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

// BindInto is generic sugar for callers outside router.
func BindInto[T any](getter ValueGetter, tag string, opts ...Option) (T, error) {
	var result T
	err := Bind(&result, getter, tag, opts...)
	return result, err
}
