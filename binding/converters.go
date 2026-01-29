// Copyright 2026 The Rivaas Authors
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
	"strings"
	"time"
)

// TimeConverter returns a converter for time.Time with custom layouts.
// The converter tries each layout in order until one succeeds.
//
// This is useful when you need to support time formats beyond the default
// layouts (RFC3339, DateOnly, DateTime, etc.).
//
// Example:
//
//	binder := binding.MustNew(
//	    binding.WithConverter(binding.TimeConverter(
//	        "01/02/2006",        // US format
//	        "02/01/2006",        // European format
//	        "2006-01-02 15:04",  // Custom datetime
//	    )),
//	)
func TimeConverter(layouts ...string) func(string) (time.Time, error) {
	if len(layouts) == 0 {
		return func(s string) (time.Time, error) {
			return time.Time{}, fmt.Errorf("no time layouts provided")
		}
	}

	return func(s string) (time.Time, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return time.Time{}, ErrEmptyTimeValue
		}

		var lastErr error
		for _, layout := range layouts {
			if t, err := time.Parse(layout, s); err == nil {
				return t, nil
			} else {
				lastErr = err
			}
		}

		return time.Time{}, fmt.Errorf("unable to parse time %q (tried %d layouts): %w",
			s, len(layouts), lastErr)
	}
}

// DurationConverter returns a converter for time.Duration with optional aliases.
// It supports both standard duration strings ("1h30m") and custom aliases.
//
// This is useful for providing user-friendly duration names like "fast", "slow",
// or "default" that map to specific durations.
//
// Example:
//
//	binder := binding.MustNew(
//	    binding.WithConverter(binding.DurationConverter(map[string]time.Duration{
//	        "fast":    100 * time.Millisecond,
//	        "normal":  1 * time.Second,
//	        "slow":    5 * time.Second,
//	        "default": 30 * time.Second,
//	    })),
//	)
//
// The converter first checks aliases, then falls back to time.ParseDuration.
func DurationConverter(aliases map[string]time.Duration) func(string) (time.Duration, error) {
	return func(s string) (time.Duration, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, fmt.Errorf("empty duration value")
		}

		// Check aliases first (case-insensitive)
		if aliases != nil {
			lower := strings.ToLower(s)
			for alias, duration := range aliases {
				if strings.ToLower(alias) == lower {
					return duration, nil
				}
			}
		}

		// Fallback to standard duration parsing
		d, err := time.ParseDuration(s)
		if err != nil {
			if aliases != nil {
				validAliases := make([]string, 0, len(aliases))
				for alias := range aliases {
					validAliases = append(validAliases, alias)
				}
				return 0, fmt.Errorf("invalid duration %q: not a valid duration string or alias (valid aliases: %s)",
					s, strings.Join(validAliases, ", "))
			}
			return 0, fmt.Errorf("invalid duration: %w", err)
		}

		return d, nil
	}
}

// EnumConverter creates a converter that validates string values against
// a set of allowed values. It returns an error if the value is not in the
// allowed set.
//
// This is useful for fields with a fixed set of valid values (enums, status
// codes, etc.).
//
// Example:
//
//	type Status string
//
//	const (
//	    StatusActive   Status = "active"
//	    StatusPending  Status = "pending"
//	    StatusDisabled Status = "disabled"
//	)
//
//	binder := binding.MustNew(
//	    binding.WithConverter(binding.EnumConverter(
//	        StatusActive,
//	        StatusPending,
//	        StatusDisabled,
//	    )),
//	)
//
// The comparison is case-insensitive for better UX.
func EnumConverter[T ~string](allowed ...T) func(string) (T, error) {
	if len(allowed) == 0 {
		return func(s string) (T, error) {
			return T(""), fmt.Errorf("no allowed values provided")
		}
	}

	// Build lowercase map for case-insensitive matching
	allowedMap := make(map[string]T, len(allowed))
	for _, val := range allowed {
		allowedMap[strings.ToLower(string(val))] = val
	}

	return func(s string) (T, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return T(""), fmt.Errorf("empty value")
		}

		// Case-insensitive lookup
		lower := strings.ToLower(s)
		if val, ok := allowedMap[lower]; ok {
			return val, nil
		}

		// Build error message with allowed values
		allowedStrs := make([]string, 0, len(allowed))
		for _, val := range allowed {
			allowedStrs = append(allowedStrs, string(val))
		}

		return T(""), fmt.Errorf("invalid value %q: must be one of: %s",
			s, strings.Join(allowedStrs, ", "))
	}
}

// BoolConverter creates a boolean converter with custom truthy and falsy values.
// This allows you to accept non-standard boolean representations beyond the
// default (true/false, yes/no, 1/0, on/off).
//
// Example:
//
//	binder := binding.MustNew(
//	    binding.WithConverter(binding.BoolConverter(
//	        []string{"enabled", "active", "on"},  // truthy values
//	        []string{"disabled", "inactive", "off"},  // falsy values
//	    )),
//	)
//
// The comparison is case-insensitive. Empty strings default to false.
func BoolConverter(truthy, falsy []string) func(string) (bool, error) {
	// Build case-insensitive lookup sets
	truthySet := make(map[string]bool, len(truthy))
	for _, val := range truthy {
		truthySet[strings.ToLower(val)] = true
	}

	falsySet := make(map[string]bool, len(falsy))
	for _, val := range falsy {
		falsySet[strings.ToLower(val)] = true
	}

	return func(s string) (bool, error) {
		s = strings.TrimSpace(s)
		lower := strings.ToLower(s)

		// Check truthy values
		if truthySet[lower] {
			return true, nil
		}

		// Check falsy values (empty string defaults to false)
		if lower == "" || falsySet[lower] {
			return false, nil
		}

		// Build error message
		allValues := make([]string, 0, len(truthy)+len(falsy))
		allValues = append(allValues, truthy...)
		allValues = append(allValues, falsy...)

		return false, fmt.Errorf("%w: %q (accepted values: %s)",
			ErrInvalidBooleanValue, s, strings.Join(allValues, ", "))
	}
}
