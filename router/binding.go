package router

// This file contains request data binding methods for the Context type.
// These methods parse and bind request data (body, query, params, cookies, headers)
// to Go structs using reflection and struct tags.

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BindError represents a binding error with detailed context about what failed.
type BindError struct {
	Field string // Field name that failed to bind
	Tag   string // Struct tag name (json, query, params, etc.)
	Value string // The value that failed to convert
	Type  string // Expected Go type
	Err   error  // Underlying conversion error
}

// Error returns a formatted error message.
func (e *BindError) Error() string {
	return fmt.Sprintf("binding field %q (tag:%s): failed to convert %q to %s: %v",
		e.Field, e.Tag, e.Value, e.Type, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As compatibility.
func (e *BindError) Unwrap() error {
	return e.Err
}

// fieldInfo stores cached information about a struct field for efficient binding.
type fieldInfo struct {
	index        []int        // Field index path (supports nested structs)
	name         string       // Struct field name
	tagName      string       // Tag value (e.g., "user_id" from `query:"user_id"`)
	kind         reflect.Kind // Field type
	fieldType    reflect.Type // Full type information
	isPtr        bool         // Whether field is a pointer type
	isSlice      bool         // Whether field is a slice type
	isMap        bool         // Whether field is a map type
	isStruct     bool         // Whether field is a nested struct
	elemKind     reflect.Kind // Element type for slices
	enumValues   string       // Comma-separated enum values from tag
	defaultValue string       // Default value from tag
}

// structInfo holds cached parsing information for a struct type.
type structInfo struct {
	fields []fieldInfo
}

var (
	// typeCache caches parsed struct information to avoid repeated reflection
	typeCache   = make(map[reflect.Type]map[string]*structInfo)
	typeCacheMu sync.RWMutex

	// Type references for special type handling
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	timeType            = reflect.TypeOf(time.Time{})
	durationType        = reflect.TypeOf(time.Duration(0))
	urlType             = reflect.TypeOf(url.URL{})
	ipType              = reflect.TypeOf(net.IP{})
	ipNetType           = reflect.TypeOf(net.IPNet{})
	regexpType          = reflect.TypeOf(regexp.Regexp{})
)

// WarmupBindingCache pre-parses struct types to populate the type cache.
// This eliminates first-call reflection overhead for known request types.
// Call this during application startup after defining your structs.
//
// Example:
//
//	type UserRequest struct { ... }
//	type SearchParams struct { ... }
//
//	router.WarmupBindingCache(
//	    UserRequest{},
//	    SearchParams{},
//	)
func WarmupBindingCache(types ...any) {
	for _, t := range types {
		typ := reflect.TypeOf(t)
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		if typ.Kind() != reflect.Struct {
			continue
		}

		// Parse for all common tag types
		tagTypes := []string{"json", "query", "params", "form", "cookie", "header"}
		for _, tag := range tagTypes {
			getStructInfo(typ, tag)
		}
	}
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
//
// Example:
//
//	type CreateUserRequest struct {
//	    Name  string `json:"name" form:"name"`
//	    Email string `json:"email" form:"email"`
//	    Age   int    `json:"age" form:"age"`
//	}
//
//	var req CreateUserRequest
//	if err := c.BindBody(&req); err != nil {
//	    c.JSON(400, map[string]string{"error": err.Error()})
//	    return
//	}
func (c *Context) BindBody(out any) error {
	contentType := c.Request.Header.Get("Content-Type")

	// Extract base content type (remove parameters)
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(strings.ToLower(contentType))

	switch contentType {
	case "application/json", "":
		// Default to JSON if no content type specified
		return c.BindJSON(out)
	case "application/x-www-form-urlencoded":
		return c.BindForm(out)
	case "multipart/form-data":
		return c.BindForm(out)
	default:
		return fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// BindJSON binds JSON request body to a struct.
// Uses the standard encoding/json package with json struct tags.
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
		return errors.New("request body is nil")
	}

	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	return nil
}

// BindForm binds form data to a struct.
// Handles both application/x-www-form-urlencoded and multipart/form-data.
// Uses form struct tags.
//
// Example:
//
//	type LoginForm struct {
//	    Username string `form:"username"`
//	    Password string `form:"password"`
//	}
//
//	var form LoginForm
//	if err := c.BindForm(&form); err != nil {
//	    return err
//	}
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

	return bind(out, &formGetter{c.Request.Form}, "form")
}

// BindQuery binds query parameters to a struct.
// Uses query struct tags.
//
// Example:
//
//	type SearchParams struct {
//	    Query    string `query:"q"`
//	    Page     int    `query:"page"`
//	    PageSize int    `query:"page_size"`
//	}
//
//	var params SearchParams
//	if err := c.BindQuery(&params); err != nil {
//	    return err
//	}
func (c *Context) BindQuery(out any) error {
	return bind(out, &queryGetter{c.Request.URL.Query()}, "query")
}

// BindParams binds URL path parameters to a struct.
// Uses params struct tags.
//
// Example:
//
//	type UserParams struct {
//	    ID     int    `params:"id"`
//	    Action string `params:"action"`
//	}
//
//	// Route: /users/:id/:action
//	var params UserParams
//	if err := c.BindParams(&params); err != nil {
//	    return err
//	}
func (c *Context) BindParams(out any) error {
	// Build params map from both array-based params and map-based params
	allParams := make(map[string]string, c.paramCount)

	// Copy from array (fast path for ≤8 params)
	for i := range c.paramCount {
		allParams[c.paramKeys[i]] = c.paramValues[i]
	}

	// Copy from map (fallback for >8 params)
	for k, v := range c.Params {
		allParams[k] = v
	}

	return bind(out, &paramsGetter{allParams}, "params")
}

// BindCookies binds cookies to a struct.
// Uses cookie struct tags. Cookie values are automatically URL-unescaped.
//
// Example:
//
//	type SessionCookies struct {
//	    SessionID string `cookie:"session_id"`
//	    Theme     string `cookie:"theme"`
//	}
//
//	var cookies SessionCookies
//	if err := c.BindCookies(&cookies); err != nil {
//	    return err
//	}
func (c *Context) BindCookies(out any) error {
	return bind(out, &cookieGetter{c.Request.Cookies()}, "cookie")
}

// BindHeaders binds request headers to a struct.
// Uses header struct tags. Header names are case-insensitive per HTTP spec.
//
// Example:
//
//	type RequestHeaders struct {
//	    UserAgent string `header:"User-Agent"`
//	    Token     string `header:"Authorization"`
//	    Accept    string `header:"Accept"`
//	}
//
//	var headers RequestHeaders
//	if err := c.BindHeaders(&headers); err != nil {
//	    return err
//	}
func (c *Context) BindHeaders(out any) error {
	return bind(out, &headerGetter{c.Request.Header}, "header")
}

// valueGetter abstracts different data sources (query, params, cookies, headers, form).
type valueGetter interface {
	Get(key string) string
	GetAll(key string) []string
	Has(key string) bool
}

// queryGetter implements valueGetter for URL query parameters.
type queryGetter struct {
	values url.Values
}

func (q *queryGetter) Get(key string) string      { return q.values.Get(key) }
func (q *queryGetter) GetAll(key string) []string { return q.values[key] }
func (q *queryGetter) Has(key string) bool        { return q.values.Has(key) }

// paramsGetter implements valueGetter for URL path parameters.
type paramsGetter struct {
	params map[string]string
}

func (p *paramsGetter) Get(key string) string { return p.params[key] }
func (p *paramsGetter) GetAll(key string) []string {
	if val, ok := p.params[key]; ok {
		return []string{val}
	}
	return nil
}
func (p *paramsGetter) Has(key string) bool {
	_, ok := p.params[key]
	return ok
}

// cookieGetter implements valueGetter for cookies.
type cookieGetter struct {
	cookies []*http.Cookie
}

func (cg *cookieGetter) Get(key string) string {
	for _, cookie := range cg.cookies {
		if cookie.Name == key {
			// URL-decode cookie value (matching SetCookie behavior)
			if val, err := url.QueryUnescape(cookie.Value); err == nil {
				return val
			}
			return cookie.Value
		}
	}
	return ""
}

func (cg *cookieGetter) GetAll(key string) []string {
	var values []string
	for _, cookie := range cg.cookies {
		if cookie.Name == key {
			if val, err := url.QueryUnescape(cookie.Value); err == nil {
				values = append(values, val)
			} else {
				values = append(values, cookie.Value)
			}
		}
	}
	return values
}

func (cg *cookieGetter) Has(key string) bool {
	for _, cookie := range cg.cookies {
		if cookie.Name == key {
			return true
		}
	}
	return false
}

// headerGetter implements valueGetter for HTTP headers.
type headerGetter struct {
	headers http.Header
}

func (h *headerGetter) Get(key string) string      { return h.headers.Get(key) }
func (h *headerGetter) GetAll(key string) []string { return h.headers.Values(key) }
func (h *headerGetter) Has(key string) bool        { return h.headers.Get(key) != "" }

// formGetter implements valueGetter for form data.
type formGetter struct {
	values url.Values
}

func (f *formGetter) Get(key string) string      { return f.values.Get(key) }
func (f *formGetter) GetAll(key string) []string { return f.values[key] }
func (f *formGetter) Has(key string) bool        { return f.values.Has(key) }

// bind is the core binding function that uses reflection to map values to struct fields.
func bind(out any, getter valueGetter, tagName string) error {
	// Validate output is a pointer to struct
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Ptr {
		return errors.New("out must be a pointer to struct")
	}

	if rv.IsNil() {
		return errors.New("out pointer is nil")
	}

	elem := rv.Elem()
	if elem.Kind() != reflect.Struct {
		return errors.New("out must be a pointer to struct")
	}

	// Get cached struct info
	info := getStructInfo(elem.Type(), tagName)

	// Bind each field
	for _, field := range info.fields {
		// Get the field value by index path
		fieldValue := elem.FieldByIndex(field.index)
		if !fieldValue.CanSet() {
			continue // Skip unexported fields
		}

		// Handle map fields
		if field.isMap {
			if err := setMapField(fieldValue, getter, field.tagName, field.fieldType); err != nil {
				return &BindError{
					Field: field.name,
					Tag:   tagName,
					Value: "",
					Type:  fieldValue.Type().String(),
					Err:   err,
				}
			}
			continue
		}

		// Handle nested struct fields
		if field.isStruct {
			if err := setNestedStruct(fieldValue, getter, field.tagName, tagName); err != nil {
				return &BindError{
					Field: field.name,
					Tag:   tagName,
					Value: "",
					Type:  fieldValue.Type().String(),
					Err:   err,
				}
			}
			continue
		}

		// Check if value exists or use default
		value := getter.Get(field.tagName)
		hasValue := getter.Has(field.tagName)

		// Apply default value if no value provided and default is specified
		if !hasValue && field.defaultValue != "" {
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
			if err := setSliceField(fieldValue, values); err != nil {
				return &BindError{
					Field: field.name,
					Tag:   tagName,
					Value: strings.Join(values, ","),
					Type:  fieldValue.Type().String(),
					Err:   err,
				}
			}
			continue
		}

		// Handle single value fields (value already retrieved above)

		// Enum validation
		if field.enumValues != "" {
			if err := validateEnum(value, field.enumValues); err != nil {
				return &BindError{
					Field: field.name,
					Tag:   tagName,
					Value: value,
					Type:  fieldValue.Type().String(),
					Err:   err,
				}
			}
		}

		if err := setField(fieldValue, value, field.isPtr); err != nil {
			return &BindError{
				Field: field.name,
				Tag:   tagName,
				Value: value,
				Type:  fieldValue.Type().String(),
				Err:   err,
			}
		}
	}

	return nil
}

// getStructInfo retrieves cached struct information or parses it.
func getStructInfo(t reflect.Type, tagName string) *structInfo {
	// Fast path: read lock for cache lookup
	typeCacheMu.RLock()
	if tagCache, ok := typeCache[t]; ok {
		if info, ok := tagCache[tagName]; ok {
			typeCacheMu.RUnlock()
			return info
		}
	}
	typeCacheMu.RUnlock()

	// Slow path: parse struct and cache
	typeCacheMu.Lock()
	defer typeCacheMu.Unlock()

	// Double-check after acquiring write lock
	if tagCache, ok := typeCache[t]; ok {
		if info, ok := tagCache[tagName]; ok {
			return info
		}
	}

	// Parse struct fields
	info := parseStructType(t, tagName, nil)

	// Initialize cache for this type if needed
	if typeCache[t] == nil {
		typeCache[t] = make(map[string]*structInfo)
	}
	typeCache[t][tagName] = info

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
		if tag == "" && tagName != "json" && tagName != "form" {
			// For non-standard tags, skip if not present
			continue
		}

		// Handle json/form tags with options (e.g., "name,omitempty")
		if tagName == "json" || tagName == "form" {
			if tag == "-" {
				continue // Skip fields marked with "-"
			}
			// Extract just the name part before any comma
			if idx := strings.Index(tag, ","); idx != -1 {
				tag = tag[:idx]
			}
			// Use field name if tag is empty
			if tag == "" {
				tag = field.Name
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

		// Add field info
		info.fields = append(info.fields, fieldInfo{
			index:        index,
			name:         field.Name,
			tagName:      tag,
			kind:         kind,
			fieldType:    field.Type, // Store original field type (before unwrapping pointer)
			isPtr:        isPtr,
			isSlice:      isSlice,
			isMap:        isMap,
			isStruct:     isStruct,
			elemKind:     elemKind,
			enumValues:   enumValues,
			defaultValue: defaultValue,
		})
	}

	return info
}

// setField sets a single struct field value with type conversion.
func setField(field reflect.Value, value string, isPtr bool) error {
	fieldType := field.Type()

	// Handle pointer fields
	if isPtr {
		if value == "" {
			// Leave nil for empty values
			return nil
		}

		// Create new pointer and set its value
		ptr := reflect.New(fieldType.Elem())
		if err := setFieldValue(ptr.Elem(), value); err != nil {
			return err
		}
		field.Set(ptr)
		return nil
	}

	return setFieldValue(field, value)
}

// setFieldValue sets the actual field value with enhanced type support.
func setFieldValue(field reflect.Value, value string) error {
	fieldType := field.Type()

	// Priority 1: Handle special types BEFORE checking TextUnmarshaler
	// This allows us to provide better parsing for time.Time (which implements TextUnmarshaler)
	switch fieldType {
	case timeType:
		t, err := parseTime(value)
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
			return fmt.Errorf("invalid IP address: %s", value)
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
		return field.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(value))
	}

	// Priority 3: Handle primitive types
	converted, err := convertValue(value, fieldType.Kind())
	if err != nil {
		return err
	}

	// Set the field value
	switch fieldType.Kind() {
	case reflect.String:
		field.SetString(converted.(string))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.SetInt(converted.(int64))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		field.SetUint(converted.(uint64))
	case reflect.Float32, reflect.Float64:
		field.SetFloat(converted.(float64))
	case reflect.Bool:
		field.SetBool(converted.(bool))
	default:
		return fmt.Errorf("unsupported type: %v", fieldType.Kind())
	}

	return nil
}

// setSliceField sets a slice field from multiple string values.
func setSliceField(field reflect.Value, values []string) error {
	if len(values) == 0 {
		return nil
	}

	// Create slice with appropriate capacity
	slice := reflect.MakeSlice(field.Type(), len(values), len(values))

	// Convert and set each element
	for i, val := range values {
		elem := slice.Index(i)

		// Use setFieldValue for each element to handle special types
		if err := setFieldValue(elem, val); err != nil {
			return fmt.Errorf("element %d: %w", i, err)
		}
	}

	field.Set(slice)
	return nil
}

// convertValue converts a string value to the target type.
func convertValue(value string, kind reflect.Kind) (any, error) {
	switch kind {
	case reflect.String:
		return value, nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer: %w", err)
		}
		return i, nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(value, 10, 64)
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
		b, err := parseBool(value)
		if err != nil {
			return nil, err
		}
		return b, nil

	default:
		return nil, fmt.Errorf("unsupported type: %v", kind)
	}
}

// parseBool parses various boolean string representations.
// Supports: true/false, 1/0, yes/no, on/off, t/f, y/n (case-insensitive).
func parseBool(value string) (bool, error) {
	lower := strings.ToLower(strings.TrimSpace(value))

	switch lower {
	case "true", "1", "yes", "on", "t", "y":
		return true, nil
	case "false", "0", "no", "off", "f", "n", "":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %q", value)
	}
}

// parseTime attempts to parse a time string using multiple common formats.
// Tries formats in order of likelihood for best performance.
func parseTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("empty time value")
	}

	// Common formats in order of likelihood
	formats := []string{
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

	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time %q (tried RFC3339, date-only, and other common formats)", value)
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
//
// Examples:
//
//	// Dot notation
//	?metadata.name=John&metadata.age=30
//	→ map[string]string{"name": "John", "age": "30"}
//
//	// Bracket notation
//	?scores[math]=95&scores[science]=88
//	→ map[string]int{"math": 95, "science": 88}
//
//	// Quoted keys (special chars)
//	?config["db.host"]=localhost&config["db.port"]=5432
//	→ map[string]string{"db.host": "localhost", "db.port": "5432"}
func setMapField(field reflect.Value, getter valueGetter, prefix string, fieldType reflect.Type) error {
	mapType := fieldType
	if mapType.Kind() == reflect.Ptr {
		mapType = mapType.Elem()
	}

	// Only support map[string]T
	if mapType.Key().Kind() != reflect.String {
		return fmt.Errorf("only map[string]T is supported, got %v", mapType)
	}

	// Create map if needed
	if field.IsNil() {
		field.Set(reflect.MakeMap(mapType))
	}

	prefixDot := prefix + "."
	prefixBracket := prefix + "["
	valueType := mapType.Elem()
	found := false

	// For query/form getters, check all keys for both syntaxes
	if qg, ok := getter.(*queryGetter); ok {
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
					return fmt.Errorf("invalid bracket notation in key: %s", key)
				}
				found = true
				mapKey = extractedKey
			} else {
				continue
			}

			value := qg.Get(key)

			// Convert value to map value type
			convertedValue, err := convertToType(value, valueType)
			if err != nil {
				return fmt.Errorf("key %q: %w", mapKey, err)
			}

			field.SetMapIndex(reflect.ValueOf(mapKey), convertedValue)
		}
	}

	// Also check formGetter for form data
	if fg, ok := getter.(*formGetter); ok {
		for key := range fg.values {
			var mapKey string

			if strings.HasPrefix(key, prefixDot) {
				found = true
				mapKey = strings.TrimPrefix(key, prefixDot)
			} else if strings.HasPrefix(key, prefixBracket) {
				extractedKey := extractBracketKey(key, prefix)
				if extractedKey == "" {
					return fmt.Errorf("invalid bracket notation in key: %s", key)
				}
				found = true
				mapKey = extractedKey
			} else {
				continue
			}

			value := fg.Get(key)
			convertedValue, err := convertToType(value, valueType)
			if err != nil {
				return fmt.Errorf("key %q: %w", mapKey, err)
			}

			field.SetMapIndex(reflect.ValueOf(mapKey), convertedValue)
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
					convertedValue, err := convertToType(strValue, valueType)
					if err != nil {
						return fmt.Errorf("key %q: %w", k, err)
					}
					field.SetMapIndex(reflect.ValueOf(k), convertedValue)
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
// Invalid formats (returns empty string):
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

// setNestedStruct handles binding data to nested struct fields using dot notation.
// Query syntax: ?address.street=Main&address.city=NYC
func setNestedStruct(field reflect.Value, getter valueGetter, prefix string, tagName string) error {
	// Create nested value getter that filters by prefix
	nestedGetter := &prefixGetter{
		inner:  getter,
		prefix: prefix + ".",
	}

	// Recursively bind nested struct
	return bind(field.Addr().Interface(), nestedGetter, tagName)
}

// prefixGetter filters values by prefix for nested struct/map binding.
type prefixGetter struct {
	inner  valueGetter
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
	if qg, ok := pg.inner.(*queryGetter); ok {
		for k := range qg.values {
			if k == fullKey || strings.HasPrefix(k, fullKey+".") {
				return true
			}
		}
	}
	if fg, ok := pg.inner.(*formGetter); ok {
		for k := range fg.values {
			if k == fullKey || strings.HasPrefix(k, fullKey+".") {
				return true
			}
		}
	}

	return false
}

// validateEnum validates that a value is in the allowed enum list.
func validateEnum(value string, enumValues string) error {
	if value == "" {
		return nil // Empty values skip enum validation
	}

	allowed := strings.Split(enumValues, ",")
	for _, a := range allowed {
		if strings.TrimSpace(a) == value {
			return nil
		}
	}

	return fmt.Errorf("value %q not in allowed values: %s", value, enumValues)
}

// convertToType converts a string value to the target reflect.Type.
func convertToType(value string, targetType reflect.Type) (reflect.Value, error) {
	// Handle any (interface{})
	if targetType.Kind() == reflect.Interface {
		return reflect.ValueOf(value), nil
	}

	// For concrete types, use setFieldValue logic
	temp := reflect.New(targetType).Elem()
	if err := setFieldValue(temp, value); err != nil {
		return reflect.Value{}, err
	}

	return temp, nil
}
