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

package config

import (
	"fmt"
	"reflect"
	"time"

	"github.com/spf13/cast"
)

// Get returns the value associated with the given key as type T.
// If the key is not found or cannot be converted to type T, it returns the zero value of T.
// This generic function is useful for custom types or when you need type-safe access.
//
// Example:
//
//	port := config.Get[int](cfg, "server.port")
//	timeout := config.Get[time.Duration](cfg, "timeout")
//	custom := config.Get[MyCustomType](cfg, "custom")
func Get[T any](c *Config, key string) T {
	var zero T
	if c == nil {
		return zero
	}

	val := c.getValueFromMap(key)
	if val == nil {
		return zero
	}

	// Try direct type assertion first
	if result, ok := val.(T); ok {
		return result
	}

	// Fallback to cast library for common type conversions
	// This won't work for custom types, but will handle basic types
	result, ok := convertToType[T](val)
	if ok {
		return result
	}

	return zero
}

// GetOr returns the value associated with the given key as type T.
// If the key is not found or cannot be converted to type T, it returns the provided default value.
// The type T is inferred from the default value.
//
// Example:
//
//	port := config.GetOr(cfg, "server.port", 8080)           // type inferred as int
//	host := config.GetOr(cfg, "server.host", "localhost")    // type inferred as string
//	timeout := config.GetOr(cfg, "timeout", 30*time.Second)  // type inferred as time.Duration
func GetOr[T any](c *Config, key string, defaultVal T) T {
	if c == nil {
		return defaultVal
	}

	val := c.getValueFromMap(key)
	if val == nil {
		return defaultVal
	}

	// Try direct type assertion first
	if result, ok := val.(T); ok {
		return result
	}

	// Fallback to cast library for common type conversions
	result, ok := convertToType[T](val)
	if ok {
		return result
	}

	return defaultVal
}

// GetE returns the value associated with the given key as type T, with error handling.
// If the key is not found, it returns an error.
// If the value cannot be converted to type T, it returns an error.
// This is useful when you need explicit error handling for missing or invalid configuration.
//
// Example:
//
//	port, err := config.GetE[int](cfg, "server.port")
//	if err != nil {
//	    return fmt.Errorf("failed to get port: %w", err)
//	}
//
//	custom, err := config.GetE[MyCustomType](cfg, "custom")
//	if err != nil {
//	    return fmt.Errorf("failed to get custom config: %w", err)
//	}
func GetE[T any](c *Config, key string) (T, error) {
	zero := getZeroValue[T]()
	if c == nil {
		return zero, fmt.Errorf("config instance is nil")
	}

	val := c.getValueFromMap(key)
	if val == nil {
		return zero, fmt.Errorf("key %q not found", key)
	}

	// Try direct type assertion first
	if result, ok := val.(T); ok {
		return result, nil
	}

	// Fallback to cast library for common type conversions
	result, ok := convertToType[T](val)
	if ok {
		return result, nil
	}

	return zero, fmt.Errorf("cannot convert value at key %q to type %T", key, zero)
}

// getZeroValue returns a proper zero value for type T.
// For slices and maps, it returns empty initialized values instead of nil.
func getZeroValue[T any]() T {
	var zero T
	v := reflect.ValueOf(&zero).Elem()

	// Initialize slices and maps to empty instead of nil
	switch v.Kind() {
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 0, 0))
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
	}

	return zero
}

// convertToType attempts to convert a value to type T using the cast library.
// This handles common type conversions (int, string, bool, etc.) but won't work for custom types.
func convertToType[T any](val any) (T, bool) {
	var zero T
	var result any

	// Use type switch to handle common conversions
	switch any(zero).(type) {
	case string:
		result = cast.ToString(val)
	case int:
		result = cast.ToInt(val)
	case int64:
		result = cast.ToInt64(val)
	case int32:
		result = cast.ToInt32(val)
	case int16:
		result = cast.ToInt16(val)
	case int8:
		result = cast.ToInt8(val)
	case uint:
		result = cast.ToUint(val)
	case uint64:
		result = cast.ToUint64(val)
	case uint32:
		result = cast.ToUint32(val)
	case uint16:
		result = cast.ToUint16(val)
	case uint8:
		result = cast.ToUint8(val)
	case float64:
		result = cast.ToFloat64(val)
	case float32:
		result = cast.ToFloat32(val)
	case bool:
		result = cast.ToBool(val)
	case []string:
		result = cast.ToStringSlice(val)
	case []int:
		result = cast.ToIntSlice(val)
	case map[string]any:
		result = cast.ToStringMap(val)
	case map[string]string:
		result = cast.ToStringMapString(val)
	case map[string][]string:
		result = cast.ToStringMapStringSlice(val)
	case time.Duration:
		result = cast.ToDuration(val)
	case time.Time:
		result = cast.ToTime(val)
	default:
		return zero, false
	}

	// Convert result back to T
	if typedResult, ok := result.(T); ok {
		return typedResult, true
	}

	return zero, false
}
