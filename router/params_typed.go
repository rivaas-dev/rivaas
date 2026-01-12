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

package router

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrParamMissing is returned when a required parameter is not found.
	ErrParamMissing = errors.New("parameter not found")

	// ErrParamInvalid is returned when a parameter cannot be parsed.
	ErrParamInvalid = errors.New("invalid parameter value")
)

// ParamInt parses a path parameter as an int.
// Returns an error if the parameter is missing or cannot be parsed.
//
// Example:
//
//	r.GET("/users/:id", func(c *router.Ctx) error {
//	    id, err := c.ParamInt("id")
//	    if err != nil {
//	        return c.BadRequest("id must be an integer")
//	    }
//	    // Use id...
//	})
func (c *Context) ParamInt(name string) (int, error) {
	s := c.Param(name)
	if s == "" {
		return 0, fmt.Errorf("%w: %s", ErrParamMissing, name)
	}

	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %s (%w)", ErrParamInvalid, name, err)
	}

	return val, nil
}

// ParamInt64 parses a path parameter as an int64.
// Returns an error if the parameter is missing or cannot be parsed.
func (c *Context) ParamInt64(name string) (int64, error) {
	s := c.Param(name)
	if s == "" {
		return 0, fmt.Errorf("%w: %s", ErrParamMissing, name)
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s (%w)", ErrParamInvalid, name, err)
	}

	return val, nil
}

// ParamUint parses a path parameter as a uint.
// Returns an error if the parameter is missing or cannot be parsed.
func (c *Context) ParamUint(name string) (uint, error) {
	s := c.Param(name)
	if s == "" {
		return 0, fmt.Errorf("%w: %s", ErrParamMissing, name)
	}

	val, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: %s (%w)", ErrParamInvalid, name, err)
	}

	return uint(val), nil
}

// ParamUint64 parses a path parameter as a uint64.
// Returns an error if the parameter is missing or cannot be parsed.
func (c *Context) ParamUint64(name string) (uint64, error) {
	s := c.Param(name)
	if s == "" {
		return 0, fmt.Errorf("%w: %s", ErrParamMissing, name)
	}

	val, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s (%w)", ErrParamInvalid, name, err)
	}

	return val, nil
}

// ParamFloat64 parses a path parameter as a float64.
// Returns an error if the parameter is missing or cannot be parsed.
func (c *Context) ParamFloat64(name string) (float64, error) {
	s := c.Param(name)
	if s == "" {
		return 0, fmt.Errorf("%w: %s", ErrParamMissing, name)
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s (%w)", ErrParamInvalid, name, err)
	}

	return val, nil
}

// ParamUUID parses a path parameter as a UUID (RFC 4122 format).
// Returns an error if the parameter is missing or is not a valid UUID.
func (c *Context) ParamUUID(name string) ([16]byte, error) {
	var uuid [16]byte

	s := c.Param(name)
	if s == "" {
		return uuid, fmt.Errorf("%w: %s", ErrParamMissing, name)
	}

	// Fast path: expect exactly 36 chars (32 hex + 4 hyphens)
	if len(s) != 36 {
		return uuid, fmt.Errorf("%w: %s (invalid UUID length)", ErrParamInvalid, name)
	}

	// Format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	//         0       9    14   19   24
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return uuid, fmt.Errorf("%w: %s (invalid UUID format)", ErrParamInvalid, name)
	}

	// Decode hex segments
	hexSegments := []string{
		s[0:8], s[9:13], s[14:18], s[19:23], s[24:36],
	}

	idx := 0
	for _, seg := range hexSegments {
		for i := 0; i < len(seg); i += 2 {
			high := hexToByte(seg[i])
			low := hexToByte(seg[i+1])
			if high == 255 || low == 255 {
				return uuid, fmt.Errorf("%w: %s (invalid hex in UUID)", ErrParamInvalid, name)
			}
			uuid[idx] = high<<4 | low
			idx++
		}
	}

	return uuid, nil
}

// hexToByte converts a hex character to its byte value.
// Returns 255 for invalid characters.
func hexToByte(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}

	return 255 // invalid
}

// ParamTime parses a path parameter as a time.Time using the specified layout.
// Returns an error if the parameter is missing or cannot be parsed.
//
// Example:
//
//	r.GET("/posts/:date", func(c *router.Ctx) error {
//	    date, err := c.ParamTime("date", "2006-01-02")
//	    if err != nil {
//	        return c.BadRequest("invalid date format")
//	    }
//	    // Use date...
//	})
func (c *Context) ParamTime(name, layout string) (time.Time, error) {
	s := c.Param(name)
	if s == "" {
		return time.Time{}, fmt.Errorf("%w: %s", ErrParamMissing, name)
	}

	val, err := time.Parse(layout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s (%w)", ErrParamInvalid, name, err)
	}

	return val, nil
}

// ParamIntRange parses a path parameter as an int and validates it's within the range [minVal, maxVal].
// Returns an error if the parameter is missing, cannot be parsed, or is out of range.
func (c *Context) ParamIntRange(name string, minVal, maxVal int) (int, error) {
	val, err := c.ParamInt(name)
	if err != nil {
		return 0, err
	}

	if val < minVal || val > maxVal {
		return 0, fmt.Errorf("%w: %s (value %d not in range [%d, %d])", ErrParamInvalid, name, val, minVal, maxVal)
	}

	return val, nil
}

// ParamStringMaxLen validates that a path parameter string does not exceed the maximum length.
// Returns an error if the parameter is missing or exceeds maxLen.
func (c *Context) ParamStringMaxLen(name string, maxLen int) (string, error) {
	s := c.Param(name)
	if s == "" {
		return "", fmt.Errorf("%w: %s", ErrParamMissing, name)
	}

	if len(s) > maxLen {
		return "", fmt.Errorf("%w: %s (length %d exceeds maximum %d)", ErrParamInvalid, name, len(s), maxLen)
	}

	return s, nil
}

// ParamEnum validates that a path parameter is one of the allowed values.
// Returns an error if the parameter is missing or not in the allowed list.
func (c *Context) ParamEnum(name string, allowed ...string) (string, error) {
	s := c.Param(name)
	if s == "" {
		return "", fmt.Errorf("%w: %s", ErrParamMissing, name)
	}

	if slices.Contains(allowed, s) {
		return s, nil
	}

	return "", fmt.Errorf("%w: %s (value %q not in allowed list: %v)", ErrParamInvalid, name, s, allowed)
}

// QueryInt parses a query parameter as an int, returning the default value if not present or invalid.
//
// Example:
//
//	r.GET("/users", func(c *router.Ctx) error {
//	    page := c.QueryInt("page", 1)
//	    limit := c.QueryInt("limit", 10)
//	    // Use page and limit...
//	})
func (c *Context) QueryInt(name string, def int) int {
	q := c.Request.URL.Query().Get(name)
	if q == "" {
		return def
	}

	if v, err := strconv.Atoi(q); err == nil {
		return v
	}

	return def
}

// QueryInt64 parses a query parameter as an int64, returning the default value if not present or invalid.
func (c *Context) QueryInt64(name string, def int64) int64 {
	q := c.Request.URL.Query().Get(name)
	if q == "" {
		return def
	}

	if v, err := strconv.ParseInt(q, 10, 64); err == nil {
		return v
	}

	return def
}

// QueryBool parses a query parameter as a bool, returning the default value if not present.
// Valid values: "true", "1", "yes", "on" (case-insensitive) = true; all others = false.
func (c *Context) QueryBool(name string, def bool) bool {
	q := c.Request.URL.Query().Get(name)
	if q == "" {
		return def
	}

	q = strings.ToLower(strings.TrimSpace(q))
	switch q {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return def
	}
}

// QueryFloat64 parses a query parameter as a float64, returning the default value if not present or invalid.
func (c *Context) QueryFloat64(name string, def float64) float64 {
	q := c.Request.URL.Query().Get(name)
	if q == "" {
		return def
	}

	if v, err := strconv.ParseFloat(q, 64); err == nil {
		return v
	}

	return def
}

// QueryDuration parses a query parameter as a time.Duration, returning the default value if not present or invalid.
// Supports standard Go duration format (e.g., "5s", "10m", "1h").
func (c *Context) QueryDuration(name string, def time.Duration) time.Duration {
	q := c.Request.URL.Query().Get(name)
	if q == "" {
		return def
	}

	if v, err := time.ParseDuration(q); err == nil {
		return v
	}

	return def
}

// QueryTime parses a query parameter as a time.Time using the specified layout.
// Returns the default value and false if not present or invalid.
// Returns the parsed time and true if successful.
func (c *Context) QueryTime(name, layout string, def time.Time) (time.Time, bool) {
	q := c.Request.URL.Query().Get(name)
	if q == "" {
		return def, false
	}

	if v, err := time.Parse(layout, q); err == nil {
		return v, true
	}

	return def, false
}

// QueryStrings splits a comma-separated query parameter into a slice of strings.
// Returns an empty slice if the parameter is not present.
// Whitespace around each value is trimmed.
//
// Example:
//
//	// ?tags=go,rust,python
//	tags := c.QueryStrings("tags") // Returns ["go", "rust", "python"]
func (c *Context) QueryStrings(name string) []string {
	val := c.Request.URL.Query().Get(name)
	if val == "" {
		return nil
	}

	parts := strings.Split(val, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}

	return result
}

// QueryInts parses a comma-separated query parameter into a slice of ints.
// Returns an error if any value cannot be parsed as an int.
// Returns an empty slice if the parameter is not present.
//
// Example:
//
//	// ?ids=1,2,3
//	ids, err := c.QueryInts("ids") // Returns [1, 2, 3], nil
func (c *Context) QueryInts(name string) ([]int, error) {
	val := c.Request.URL.Query().Get(name)
	if val == "" {
		return nil, nil
	}

	parts := strings.Split(val, ",")
	result := make([]int, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("%w %q: %q", ErrQueryInvalidInteger, name, p)
		}
		result = append(result, n)
	}

	return result, nil
}
