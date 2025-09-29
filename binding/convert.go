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
	"encoding"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// setField sets a single struct field value with type conversion.
// It handles pointer fields by creating a new pointer if the value is non-empty.
func setField(field reflect.Value, value string, isPtr bool, opts *Options) error {
	fieldType := field.Type()

	// Handle pointer fields
	if isPtr {
		if value == "" {
			// Leave nil for empty values
			return nil
		}

		// Create new pointer and set its value
		ptr := reflect.New(fieldType.Elem())
		if err := setFieldValue(ptr.Elem(), value, opts); err != nil {
			return err
		}
		field.Set(ptr)
		return nil
	}

	return setFieldValue(field, value, opts)
}

// setFieldValue sets the actual field value with type conversion.
// It checks custom converters first, then handles special types, TextUnmarshaler
// interface, and finally primitive types.
func setFieldValue(field reflect.Value, value string, opts *Options) error {
	fieldType := field.Type()

	// Priority 0: Custom type converters (highest priority)
	if converter := findConverter(fieldType, opts); converter != nil {
		converted, err := converter(value)
		if err != nil {
			return err
		}

		// Handle pointer vs value transparently
		if fieldType.Kind() == reflect.Ptr {
			ptr := reflect.New(fieldType.Elem())
			ptr.Elem().Set(reflect.ValueOf(converted))
			field.Set(ptr)
		} else {
			field.Set(reflect.ValueOf(converted))
		}
		return nil
	}

	// Priority 1: Handle special types BEFORE checking TextUnmarshaler
	// This allows us to provide better parsing for time.Time (which implements TextUnmarshaler)
	switch fieldType {
	case timeType:
		t, err := parseTime(value, opts)
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(t))
		return nil

	case durationType:
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		field.Set(reflect.ValueOf(d))
		return nil

	case urlType:
		u, err := url.Parse(value)
		if err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		field.Set(reflect.ValueOf(*u))
		return nil

	case ipType:
		ip := net.ParseIP(value)
		if ip == nil {
			return fmt.Errorf("%w: %s", ErrInvalidIPAddress, value)
		}
		field.Set(reflect.ValueOf(ip))
		return nil

	case ipNetType:
		_, ipnet, err := net.ParseCIDR(value)
		if err != nil {
			return fmt.Errorf("invalid CIDR notation: %w", err)
		}
		field.Set(reflect.ValueOf(*ipnet))
		return nil

	case regexpType:
		re, err := regexp.Compile(value)
		if err != nil {
			return fmt.Errorf("invalid regular expression: %w", err)
		}
		field.Set(reflect.ValueOf(*re))
		return nil
	}

	// Priority 2: Check for encoding.TextUnmarshaler interface
	// This allows custom types to define their own parsing logic
	if field.CanAddr() && field.Addr().Type().Implements(textUnmarshalerType) {
		unmarshaler, ok := field.Addr().Interface().(encoding.TextUnmarshaler)
		if !ok {
			return fmt.Errorf("%w: failed to assert TextUnmarshaler", ErrUnsupportedType)
		}
		return unmarshaler.UnmarshalText([]byte(value))
	}

	// Priority 3: Handle primitive types
	converted, err := convertValue(value, fieldType.Kind(), opts)
	if err != nil {
		return err
	}

	// Set the field value
	switch fieldType.Kind() {
	case reflect.String:
		str, ok := converted.(string)
		if !ok {
			return fmt.Errorf("%w: expected string, got %T", ErrUnsupportedType, converted)
		}
		field.SetString(str)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, ok := converted.(int64)
		if !ok {
			return fmt.Errorf("%w: expected int64, got %T", ErrUnsupportedType, converted)
		}
		field.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, ok := converted.(uint64)
		if !ok {
			return fmt.Errorf("%w: expected uint64, got %T", ErrUnsupportedType, converted)
		}
		field.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, ok := converted.(float64)
		if !ok {
			return fmt.Errorf("%w: expected float64, got %T", ErrUnsupportedType, converted)
		}
		field.SetFloat(f)
	case reflect.Bool:
		b, ok := converted.(bool)
		if !ok {
			return fmt.Errorf("%w: expected bool, got %T", ErrUnsupportedType, converted)
		}
		field.SetBool(b)
	default:
		return fmt.Errorf("%w: %v", ErrUnsupportedType, fieldType.Kind())
	}

	return nil
}

// setSliceField sets a slice field from multiple string values.
// It handles CSV mode (comma-separated values) and enforces maximum slice length limits.
func setSliceField(field reflect.Value, values []string, opts *Options) error {
	if len(values) == 0 {
		return nil
	}

	// Handle CSV mode: if single value and CSV mode enabled, split it
	if opts.SliceMode == SliceCSV && len(values) == 1 {
		split := strings.Split(values[0], ",")
		// Trim whitespace from each element
		for i := range split {
			split[i] = strings.TrimSpace(split[i])
		}
		values = split
	}

	// Enforce maximum slice length
	if opts.maxSliceLen > 0 && len(values) > opts.maxSliceLen {
		return fmt.Errorf("%w: %d > %d (use WithMaxSliceLen to increase)",
			ErrSliceExceedsMaxLength, len(values), opts.maxSliceLen)
	}

	// Create slice with appropriate capacity
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))

	// Convert and set each element
	for i, val := range values {
		elem := slice.Index(i)

		// Use setFieldValue for each element to handle special types
		if err := setFieldValue(elem, val, opts); err != nil {
			return fmt.Errorf("element %d: %w", i, err)
		}
	}

	field.Set(slice)
	return nil
}

// convertValue converts a string value to the target reflect.Kind.
// It handles strings, integers, unsigned integers, floats, and booleans.
func convertValue(value string, kind reflect.Kind, opts *Options) (any, error) {
	switch kind {
	case reflect.String:
		return value, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		base := 10
		if opts.IntBaseAuto {
			base = 0 // Auto-detect: 0x=hex, 0=octal, 0b=binary
		}
		i, err := strconv.ParseInt(value, base, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer: %w", err)
		}
		return i, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		base := 10
		if opts.IntBaseAuto {
			base = 0 // Auto-detect: 0x=hex, 0=octal, 0b=binary
		}
		u, err := strconv.ParseUint(value, base, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid unsigned integer: %w", err)
		}
		return u, nil

	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float: %w", err)
		}
		return f, nil

	case reflect.Bool:
		b, err := parseBoolGenerous(value)
		if err != nil {
			return nil, err
		}
		return b, nil

	default:
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedType, kind)
	}
}

// parseBoolGenerous parses various boolean string representations.
// It supports: true/false, 1/0, yes/no, on/off, t/f, y/n (case-insensitive).
func parseBoolGenerous(s string) (bool, error) {
	lower := strings.ToLower(strings.TrimSpace(s))
	switch lower {
	case "true", "1", "yes", "on", "t", "y":
		return true, nil
	case "false", "0", "no", "off", "f", "n", "":
		return false, nil
	default:
		return false, fmt.Errorf("%w: %q", ErrInvalidBooleanValue, s)
	}
}

// parseTime attempts to parse a time string using multiple formats.
// It tries default formats first (RFC3339, date-only, etc.), then custom layouts
// from options. Returns an error if no format matches.
func parseTime(value string, opts *Options) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, ErrEmptyTimeValue
	}

	// Try default formats first (most common)
	defaultFormats := []string{
		time.RFC3339,          // 2024-01-15T10:30:00Z (ISO 8601)
		time.RFC3339Nano,      // with nanoseconds
		"2006-01-02",          // Date only: 2024-01-15
		"2006-01-02 15:04:05", // DateTime: 2024-01-15 10:30:00
		time.RFC1123,          // Mon, 02 Jan 2006 15:04:05 MST
		time.RFC1123Z,         // Mon, 02 Jan 2006 15:04:05 -0700
		time.RFC822,           // 02 Jan 06 15:04 MST
		time.RFC822Z,          // 02 Jan 06 15:04 -0700
		time.RFC850,           // Monday, 02-Jan-06 15:04:05 MST
		"2006-01-02T15:04:05", // DateTime without timezone
	}

	// Try default formats
	for _, format := range defaultFormats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}

	// Try custom layouts from options
	for _, layout := range opts.TimeLayouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("%w %q (tried RFC3339, date-only, and other common formats)", ErrUnableToParseTime, value)
}

// convertToType converts a string value to the target reflect.Type.
// It handles interface{} types and delegates to setFieldValue for concrete types.
func convertToType(value string, targetType reflect.Type, opts *Options) (reflect.Value, error) {
	// Handle any (interface{})
	if targetType.Kind() == reflect.Interface {
		return reflect.ValueOf(value), nil
	}

	// For concrete types, use setFieldValue logic
	temp := reflect.New(targetType).Elem()
	if err := setFieldValue(temp, value, opts); err != nil {
		return reflect.Value{}, err
	}

	return temp, nil
}

// validateEnum validates that a value is in the allowed enum list.
// Empty values skip validation. The enumValues string is comma-separated.
func validateEnum(value string, enumValues string) error {
	if value == "" {
		return nil // Empty values skip enum validation
	}

	for a := range strings.SplitSeq(enumValues, ",") {
		if strings.TrimSpace(a) == value {
			return nil
		}
	}

	return fmt.Errorf("%w %q: %s", ErrValueNotInAllowedValues, value, enumValues)
}

// setMapField handles binding data to map fields using dot or bracket notation.
//
// SUPPORTED SYNTAXES:
//
// 1. Dot Notation (clean, recommended):
//
//	?mapName.key1=value1&mapName.key2=value2
//
// 2. Bracket Notation (PHP-style):
//
//	?mapName[key1]=value1&mapName[key2]=value2
//
// 3. Quoted Keys (for special characters):
//
//	?mapName["user.name"]=John&mapName['user-email']=test@example.com
//
// 4. Mixed Syntax (both work together):
//
//	?metadata.key1=val1&metadata[key2]=val2
//
// Supported map types:
//   - map[string]string, map[string]int, map[string]float64
//   - map[string]bool, map[string]time.Time, map[string]time.Duration
//   - map[string]net.IP, map[string]any
func setMapField(field reflect.Value, getter ValueGetter, prefix string, fieldType reflect.Type, opts *Options) error {
	mapType := fieldType
	isPtr := mapType.Kind() == reflect.Ptr
	if isPtr {
		mapType = mapType.Elem() // Extract element type from pointer
	}

	// Only support map[string]T
	if mapType.Key().Kind() != reflect.String {
		return fmt.Errorf("%w, got %v", ErrOnlyMapStringTSupported, mapType)
	}

	// Estimate capacity and enforce limit
	capacity := estimateMapCapacity(getter, prefix)
	if opts.maxMapSize > 0 && capacity > opts.maxMapSize {
		return fmt.Errorf("%w: %d > %d (use WithMaxMapSize to increase)",
			ErrMapExceedsMaxSize, capacity, opts.maxMapSize)
	}

	// Get the actual map value (dereference if pointer)
	mapField := field
	if isPtr {
		if field.IsNil() {
			ptr := reflect.New(mapType)
			ptr.Elem().Set(reflect.MakeMap(mapType))
			field.Set(ptr)
		}
		mapField = field.Elem()
	} else {
		// Create map with estimated capacity
		if mapField.IsNil() {
			mapField.Set(reflect.MakeMapWithSize(mapType, capacity))
		}
	}

	prefixDot := prefix + "."
	prefixBracket := prefix + "["
	valueType := mapType.Elem()
	found := false
	entryCount := 0

	// For query/form getters, check all keys for both syntaxes
	if qg, ok := getter.(*QueryGetter); ok {
		for key := range qg.values {
			var mapKey string

			// Pattern 1: Dot notation (?map.key=value)
			if strings.HasPrefix(key, prefixDot) {
				found = true
				mapKey = strings.TrimPrefix(key, prefixDot)

				// Pattern 2: Bracket notation (?map[key]=value)
			} else if strings.HasPrefix(key, prefixBracket) {
				extractedKey := extractBracketKey(key, prefix)
				if extractedKey == "" {
					return fmt.Errorf("%w: %s", ErrInvalidBracketNotation, key)
				}
				found = true
				mapKey = extractedKey
			} else {
				continue
			}

			// Enforce limit during iteration
			if opts.maxMapSize > 0 {
				entryCount++
				if entryCount > opts.maxMapSize {
					return fmt.Errorf("%w: %d > %d (use WithMaxMapSize to increase)",
						ErrMapExceedsMaxSize, entryCount, opts.maxMapSize)
				}
			}

			value := qg.Get(key)

			// Convert value to map value type
			convertedValue, err := convertToType(value, valueType, opts)
			if err != nil {
				return fmt.Errorf("key %q: %w", mapKey, err)
			}

			mapField.SetMapIndex(reflect.ValueOf(mapKey), convertedValue)
		}
	}

	// Also check formGetter for form data
	if fg, ok := getter.(*FormGetter); ok {
		for key := range fg.values {
			var mapKey string

			if strings.HasPrefix(key, prefixDot) {
				found = true
				mapKey = strings.TrimPrefix(key, prefixDot)
			} else if strings.HasPrefix(key, prefixBracket) {
				extractedKey := extractBracketKey(key, prefix)
				if extractedKey == "" {
					return fmt.Errorf("%w: %s", ErrInvalidBracketNotation, key)
				}
				found = true
				mapKey = extractedKey
			} else {
				continue
			}

			// Enforce limit during iteration
			if opts.maxMapSize > 0 {
				entryCount++
				if entryCount > opts.maxMapSize {
					return fmt.Errorf("%w: %d > %d (use WithMaxMapSize to increase)",
						ErrMapExceedsMaxSize, entryCount, opts.maxMapSize)
				}
			}

			value := fg.Get(key)
			convertedValue, err := convertToType(value, valueType, opts)
			if err != nil {
				return fmt.Errorf("key %q: %w", mapKey, err)
			}

			mapField.SetMapIndex(reflect.ValueOf(mapKey), convertedValue)
		}
	}

	// If no dot/bracket keys found, try JSON string parsing as fallback
	if !found && getter.Has(prefix) {
		jsonValue := getter.Get(prefix)
		if jsonValue != "" {
			// Try to parse as JSON object
			tempMap := make(map[string]any)
			if err := json.Unmarshal([]byte(jsonValue), &tempMap); err == nil {
				for k, v := range tempMap {
					// Convert interface{} to string for setting
					strValue := fmt.Sprint(v)
					convertedValue, err := convertToType(strValue, valueType, opts)
					if err != nil {
						return fmt.Errorf("key %q: %w", k, err)
					}
					mapField.SetMapIndex(reflect.ValueOf(k), convertedValue)
				}
			}
		}
	}

	return nil
}

// extractBracketKey extracts the map key from bracket notation.
//
// Supported formats:
//   - "metadata[name]" → "name"
//   - "metadata[\"user.name\"]" → "user.name"
//   - "metadata['key-with-dash']" → "key-with-dash"
//
// Invalid formats return an empty string:
//   - "metadata[]" → "" (empty brackets, array notation)
//   - "metadata[unclosed" → "" (no closing bracket)
//   - "metadata[a][b]" → "" (nested brackets, array notation)
func extractBracketKey(fullKey, prefix string) string {
	// Remove prefix
	if !strings.HasPrefix(fullKey, prefix+"[") {
		return ""
	}

	after := strings.TrimPrefix(fullKey, prefix+"[")

	// Find closing bracket
	closeBracket := strings.Index(after, "]")
	if closeBracket == -1 {
		return "" // Malformed - no closing bracket
	}

	key := after[:closeBracket]

	// Check for array notation patterns
	// Empty brackets: metadata[]
	if key == "" {
		return "" // This is array notation, not map
	}

	// Check for nested brackets after the first closing bracket: metadata[key1][key2]
	afterClose := after[closeBracket:]
	if strings.Contains(afterClose, "[") {
		return "" // This is nested array notation
	}

	// Handle quoted keys: ["key"] or ['key']
	// Remove surrounding quotes (both single and double)
	key = strings.Trim(key, `"'`)

	// Validate key is not empty after trimming
	if key == "" {
		return ""
	}

	return key
}

// setNestedStructWithDepth handles nested struct binding with depth tracking.
// It creates a prefix getter that filters values by the prefix (e.g., "address.")
// and recursively binds the nested struct. Query syntax: ?address.street=Main&address.city=NYC
func setNestedStructWithDepth(field reflect.Value, getter ValueGetter, prefix string,
	tagName string, opts *Options, depth int) error {

	// Create nested value getter that filters by prefix
	nestedGetter := &prefixGetter{
		inner:  getter,
		prefix: prefix + ".",
	}

	// Recursively bind nested struct with incremented depth
	return bindFieldsWithDepth(field, nestedGetter, tagName,
		getStructInfo(field.Type(), tagName), opts, depth)
}

// prefixGetter filters values by prefix for nested struct/map binding.
// It prepends the prefix to all key lookups, enabling dot-notation access
// to nested structures.
type prefixGetter struct {
	inner  ValueGetter
	prefix string
}

func (pg *prefixGetter) Get(key string) string {
	return pg.inner.Get(pg.prefix + key)
}

func (pg *prefixGetter) GetAll(key string) []string {
	return pg.inner.GetAll(pg.prefix + key)
}

func (pg *prefixGetter) Has(key string) bool {
	// Check if any key with this prefix exists
	fullKey := pg.prefix + key

	// Direct check first
	if pg.inner.Has(fullKey) {
		return true
	}

	// For nested structs/maps, check if any key starts with prefix
	if qg, ok := pg.inner.(*QueryGetter); ok {
		for k := range qg.values {
			if k == fullKey || strings.HasPrefix(k, fullKey+".") {
				return true
			}
		}
	}
	if fg, ok := pg.inner.(*FormGetter); ok {
		for k := range fg.values {
			if k == fullKey || strings.HasPrefix(k, fullKey+".") {
				return true
			}
		}
	}

	return false
}

// findConverter locates a registered converter for the given type.
// It checks for direct matches, pointer normalization (T vs *T), and interface
// implementations. Returns nil if no converter is found.
func findConverter(fieldType reflect.Type, opts *Options) TypeConverter {
	if opts.TypeConverters == nil {
		return nil
	}

	// Direct match: registered for exact type
	if conv, ok := opts.TypeConverters[fieldType]; ok {
		return conv
	}

	// Pointer normalization: if field is *T, check for converter registered for T
	if fieldType.Kind() == reflect.Ptr {
		if conv, ok := opts.TypeConverters[fieldType.Elem()]; ok {
			return conv // Will be wrapped transparently by caller
		}
	}

	// Interface match: check if any registered interface is implemented by this type
	for regType, conv := range opts.TypeConverters {
		if regType.Kind() == reflect.Interface && fieldType.Implements(regType) {
			return conv
		}
		// Also check pointer receiver
		if regType.Kind() == reflect.Interface && reflect.PointerTo(fieldType).Implements(regType) {
			return conv
		}
	}

	return nil
}

// estimateMapCapacity estimates the number of map entries for a given prefix.
// It uses the approxSizer interface if available, otherwise returns a default capacity.
func estimateMapCapacity(getter ValueGetter, prefix string) int {
	// Check if getter implements approxSizer capability
	if sizer, ok := getter.(approxSizer); ok {
		if count := sizer.ApproxLen(prefix); count > 0 {
			return count
		}
	}

	// Fallback to reasonable default
	return 8
}
