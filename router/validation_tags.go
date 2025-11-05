package router

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-playground/validator/v10"
)

var (
	tagValidator      *validator.Validate
	tagValidatorOnce  sync.Once
	tagValidatorMu    sync.Mutex
	validationsFrozen atomic.Bool

	// Pre-compiled regexes for performance
	reUsername = regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)
	reSlug     = regexp.MustCompile(`^[a-z0-9-]+$`)

	// Path cache: Type -> namespace -> JSON path
	pathCache sync.Map // map[reflect.Type]*sync.Map[string]string

	// Field map cache: Type -> JSON field name -> field index
	// This avoids linear search when resolving paths
	fieldMapCache sync.Map // map[reflect.Type]map[string]int
)

// Performance Characteristics:
//
// Tag validation has O(n) complexity where n is the number of fields to validate.
//
// Optimizations:
//   - Field path resolution is cached per struct type (getFieldMap)
//   - Namespace-to-JSON-path conversion is cached per type (getCachedJSONPath)
//   - Partial validation only validates leaf fields that are present
//   - Configurable max fields limit prevents DoS (WithMaxFields)
//
// Memory usage:
//   - Path cache: ~100 bytes per unique struct type + ~50 bytes per namespace
//   - Field map cache: ~24 bytes per struct field
//   - Validator instance: ~1KB (shared, lazy initialized)
//
// Thread safety:
//   - initTagValidator uses sync.Once for safe initialization
//   - RegisterTag must be called before first validation (frozen after use)
//   - All caches use sync.Map for concurrent access

// initTagValidator initializes the tag validator (private, lazy).
// This function uses sync.Once to ensure thread-safe, single initialization.
func initTagValidator() {
	tagValidatorOnce.Do(func() {
		tagValidator = validator.New(validator.WithRequiredStructEnabled())

		// Use json tags as field names for better error messages
		tagValidator.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := fld.Tag.Get("json")
			if name == "-" {
				return ""
			}
			if idx := strings.Index(name, ","); idx != -1 {
				name = name[:idx]
			}
			if name == "" {
				return fld.Name
			}
			return name
		})

		registerBuiltinValidators(tagValidator)
	})

	// Freeze after first use
	validationsFrozen.Store(true)
}

// RegisterTag allows custom tag registration.
// MUST be called at startup before any validation.
// After the first validation, registration is frozen for thread safety.
//
// Example:
//
//	router.RegisterTag("custom_tag", func(fl validator.FieldLevel) bool {
//	    return fl.Field().String() == "valid"
//	})
func RegisterTag(name string, fn validator.Func) error {
	tagValidatorMu.Lock()
	defer tagValidatorMu.Unlock()

	// Check frozen status inside mutex to avoid race condition
	if validationsFrozen.Load() {
		return fmt.Errorf("cannot register validators after first use")
	}

	initTagValidator()

	return tagValidator.RegisterValidation(name, fn)
}

func registerBuiltinValidators(v *validator.Validate) {
	v.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		return reUsername.MatchString(fl.Field().String())
	})

	v.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
		return reSlug.MatchString(fl.Field().String())
	})

	v.RegisterValidation("strong_password", func(fl validator.FieldLevel) bool {
		return len(fl.Field().String()) >= 8
	})
}

// validateWithTags validates using go-playground/validator struct tags.
func validateWithTags(v any, config *validationConfig) error {
	initTagValidator()

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil
	}

	// Partial mode: validate only leaf fields that are present
	if config.partial && config.presence != nil {
		return validatePartialLeafsOnly(v, config)
	}

	// Full validation
	err := tagValidator.Struct(v)
	if err == nil {
		return nil
	}

	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		return formatTagErrors(validationErrs, v, config)
	}

	return err
}

// validatePartialLeafsOnly validates only leaf fields present in request.
// This avoids enforcing "required" on nested fields that weren't provided.
func validatePartialLeafsOnly(v any, config *validationConfig) error {
	leaves := config.presence.LeafPaths()
	if len(leaves) == 0 {
		return nil
	}

	// Sanity cap to prevent pathological inputs
	maxLeaves := 10000 // default
	if config.maxFields > 0 {
		maxLeaves = config.maxFields
	}
	if len(leaves) > maxLeaves {
		leaves = leaves[:maxLeaves]
	}

	var result ValidationErrors
	structType := reflect.TypeOf(v)
	for structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	for _, path := range leaves {
		// Resolve field value and struct field
		fieldVal, structField, ok := resolvePath(v, path)
		if !ok {
			continue
		}

		// Get validation tag
		validateTag := structField.Tag.Get("validate")
		if validateTag == "" {
			continue
		}

		// Validate this single field
		if err := tagValidator.Var(fieldVal.Interface(), validateTag); err != nil {
			if verrs, ok := err.(validator.ValidationErrors); ok {
				// Format with proper path context
				for _, e := range verrs {
					code := "tag." + e.Tag()
					msg := getTagErrorMessage(e)

					// Redact if needed
					value := fmt.Sprint(e.Value())
					if config.redactor != nil && config.redactor(path) {
						value = "***REDACTED***"
						msg = strings.ReplaceAll(msg, fmt.Sprint(e.Value()), "***REDACTED***")
					}

					result.Add(path, code, msg, map[string]any{
						"tag":   e.Tag(),
						"param": e.Param(),
						"value": value,
					})
				}
			} else {
				result.AddError(err)
			}
		}

		// Check max errors
		if config.maxErrors > 0 && len(result.Errors) >= config.maxErrors {
			result.Truncated = true
			break
		}
	}

	if result.HasErrors() {
		result.Sort()
		return result
	}
	return nil
}

// resolvePath resolves "items.2.name" to reflect.Value and reflect.StructField.
func resolvePath(v any, path string) (reflect.Value, reflect.StructField, bool) {
	parts := strings.Split(path, ".")
	currentVal := reflect.ValueOf(v)
	var currentField reflect.StructField

	for i, part := range parts {
		// Dereference pointers
		for currentVal.Kind() == reflect.Ptr {
			if currentVal.IsNil() {
				return reflect.Value{}, reflect.StructField{}, false
			}
			currentVal = currentVal.Elem()
		}

		// Handle array/slice index
		if idx, err := strconv.Atoi(part); err == nil {
			if currentVal.Kind() == reflect.Slice || currentVal.Kind() == reflect.Array {
				if idx >= 0 && idx < currentVal.Len() {
					currentVal = currentVal.Index(idx)
					continue
				}
			}
			return reflect.Value{}, reflect.StructField{}, false
		}

		// Handle struct field
		if currentVal.Kind() == reflect.Struct {
			// Find field by JSON tag using cached field map
			structType := currentVal.Type()
			fieldMap := getFieldMap(structType)

			if fieldIndex, found := fieldMap[part]; found {
				currentField = structType.Field(fieldIndex)
				currentVal = currentVal.Field(fieldIndex)
			} else {
				return reflect.Value{}, reflect.StructField{}, false
			}

			// If this is the last part, return
			if i == len(parts)-1 {
				return currentVal, currentField, true
			}

			continue
		}

		return reflect.Value{}, reflect.StructField{}, false
	}

	return currentVal, currentField, true
}

// getJSONFieldName extracts the JSON field name from a struct field tag.
func getJSONFieldName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")
	if jsonTag == "" || jsonTag == "-" {
		return field.Name
	}
	if idx := strings.Index(jsonTag, ","); idx != -1 {
		return jsonTag[:idx]
	}
	return jsonTag
}

// getFieldMap returns a cached map of JSON field names to field indices for a struct type.
// This avoids linear search when resolving paths.
func getFieldMap(structType reflect.Type) map[string]int {
	// Check cache
	if cached, ok := fieldMapCache.Load(structType); ok {
		return cached.(map[string]int)
	}

	// Build field map
	fieldMap := make(map[string]int, structType.NumField())
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		jsonName := getJSONFieldName(field)
		if jsonName != "" && jsonName != "-" {
			fieldMap[jsonName] = i
		}
	}

	// Cache it
	fieldMapCache.Store(structType, fieldMap)
	return fieldMap
}

// formatTagErrors formats validator errors with stable codes and cached path resolution.
func formatTagErrors(errs validator.ValidationErrors, structValue any, config *validationConfig) ValidationErrors {
	var result ValidationErrors
	structType := reflect.TypeOf(structValue)
	for structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
	}

	for _, e := range errs {
		ns := e.Namespace()
		structNS := e.StructNamespace()

		// Strip top struct name
		if idx := strings.Index(structNS, "."); idx != -1 {
			ns = ns[idx+1:]
		}

		// Get or compute JSON path (cached)
		path := getCachedJSONPath(ns, structType)

		if config.fieldNameMapper != nil {
			path = config.fieldNameMapper(path)
		}

		// Stable code
		code := "tag." + e.Tag()
		msg := getTagErrorMessage(e)

		// Redact
		value := fmt.Sprint(e.Value())
		if config.redactor != nil && config.redactor(path) {
			value = "***REDACTED***"
			msg = strings.ReplaceAll(msg, fmt.Sprint(e.Value()), "***REDACTED***")
		}

		result.Add(path, code, msg, map[string]any{
			"tag":   e.Tag(),
			"param": e.Param(),
			"value": value,
		})

		if config.maxErrors > 0 && len(result.Errors) >= config.maxErrors {
			result.Truncated = true
			break
		}
	}

	result.Sort()
	return result
}

// getCachedJSONPath gets or computes JSON path with caching.
func getCachedJSONPath(ns string, structType reflect.Type) string {
	// Load type cache
	cacheVal, ok := pathCache.Load(structType)
	if !ok {
		cacheVal = &sync.Map{}
		pathCache.Store(structType, cacheVal)
	}

	typeCache := cacheVal.(*sync.Map)

	// Check if path already computed
	if cached, ok := typeCache.Load(ns); ok {
		return cached.(string)
	}

	// Compute and cache
	jsonPath := namespaceToJSONPath(ns, structType)
	typeCache.Store(ns, jsonPath)

	return jsonPath
}

// namespaceToJSONPath converts validator namespace to JSON path using struct tags.
func namespaceToJSONPath(ns string, structType reflect.Type) string {
	parts := strings.Split(ns, ".")
	result := make([]string, 0, len(parts))

	currentType := structType
	for _, part := range parts {
		// Numeric index
		if idx, err := strconv.Atoi(part); err == nil {
			result = append(result, strconv.Itoa(idx))
			if currentType.Kind() == reflect.Slice || currentType.Kind() == reflect.Array {
				currentType = currentType.Elem()
			}
			continue
		}

		// Struct field
		if currentType.Kind() == reflect.Struct {
			if field, found := currentType.FieldByName(part); found {
				jsonName := getJSONFieldName(field)
				result = append(result, jsonName)
				currentType = field.Type

				if currentType.Kind() == reflect.Ptr {
					currentType = currentType.Elem()
				}
				continue
			}
		}

		// Fallback
		result = append(result, strings.ToLower(part))
	}

	return strings.Join(result, ".")
}

// getTagErrorMessage returns a human-readable error message for a tag error.
func getTagErrorMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "url":
		return "must be a valid URL"
	case "min":
		if e.Type().Kind() == reflect.String {
			return fmt.Sprintf("must be at least %s characters", e.Param())
		}
		return fmt.Sprintf("must be at least %s", e.Param())
	case "max":
		if e.Type().Kind() == reflect.String {
			return fmt.Sprintf("must be at most %s characters", e.Param())
		}
		return fmt.Sprintf("must be at most %s", e.Param())
	case "oneof":
		return fmt.Sprintf("must be one of [%s]", e.Param())
	case "username":
		return "must be 3-20 alphanumeric characters or underscore"
	case "slug":
		return "must be lowercase letters, numbers, and hyphens"
	case "strong_password":
		return "must be at least 8 characters"
	default:
		return fmt.Sprintf("failed validation (%s)", e.Tag())
	}
}
