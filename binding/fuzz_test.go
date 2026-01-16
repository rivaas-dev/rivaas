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

//go:build !integration

package binding

import (
	"net/url"
	"reflect"
	"testing"
)

// FuzzConvertValue_Int tests integer conversion with fuzz input
func FuzzConvertValue_Int(f *testing.F) {
	// Seed corpus with known inputs
	f.Add("0")
	f.Add("1")
	f.Add("-1")
	f.Add("123456789")
	f.Add("-123456789")
	f.Add("9223372036854775807")  // max int64
	f.Add("-9223372036854775808") // min int64
	f.Add("")
	f.Add("abc")
	f.Add("123abc")
	f.Add("12.34")
	f.Add("  123  ")
	f.Add("0x1A")
	f.Add("0777")
	f.Add("0b1010")

	f.Fuzz(func(t *testing.T, input string) {
		opts := defaultConfig()
		// Should never panic, even with invalid input
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Int, opts)
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Int8, opts)
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Int16, opts)
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Int32, opts)
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Int64, opts)
	})
}

// FuzzConvertValue_Uint tests unsigned integer conversion with fuzz input
func FuzzConvertValue_Uint(f *testing.F) {
	// Seed corpus with known inputs
	f.Add("0")
	f.Add("1")
	f.Add("123456789")
	f.Add("18446744073709551615") // max uint64
	f.Add("-1")                   // negative should fail
	f.Add("")
	f.Add("abc")
	f.Add("123abc")
	f.Add("12.34")
	f.Add("  123  ")

	f.Fuzz(func(t *testing.T, input string) {
		opts := defaultConfig()
		// Should never panic, even with invalid input
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Uint, opts)
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Uint8, opts)
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Uint16, opts)
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Uint32, opts)
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Uint64, opts)
	})
}

// FuzzConvertValue_Float tests float conversion with fuzz input
func FuzzConvertValue_Float(f *testing.F) {
	// Seed corpus with known inputs
	f.Add("0")
	f.Add("0.0")
	f.Add("1.23")
	f.Add("-1.23")
	f.Add("1e10")
	f.Add("-1e-10")
	f.Add("3.14159265358979")
	f.Add("inf")
	f.Add("-inf")
	f.Add("NaN")
	f.Add("")
	f.Add("abc")
	f.Add("12.34.56")
	f.Add("  3.14  ")

	f.Fuzz(func(t *testing.T, input string) {
		opts := defaultConfig()
		// Should never panic, even with invalid input
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Float32, opts)
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Float64, opts)
	})
}

// FuzzConvertValue_Bool tests boolean conversion with fuzz input
func FuzzConvertValue_Bool(f *testing.F) {
	// Seed corpus with known inputs
	f.Add("true")
	f.Add("false")
	f.Add("TRUE")
	f.Add("FALSE")
	f.Add("True")
	f.Add("False")
	f.Add("1")
	f.Add("0")
	f.Add("yes")
	f.Add("no")
	f.Add("on")
	f.Add("off")
	f.Add("t")
	f.Add("f")
	f.Add("y")
	f.Add("n")
	f.Add("")
	f.Add("maybe")
	f.Add("2")
	f.Add("truee")
	f.Add("  true  ")

	f.Fuzz(func(t *testing.T, input string) {
		opts := defaultConfig()
		// Should never panic, even with invalid input
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = convertValue(input, reflect.Bool, opts)
	})
}

// FuzzParseBoolGenerous tests the generous bool parser
func FuzzParseBoolGenerous(f *testing.F) {
	// Seed corpus
	f.Add("true")
	f.Add("false")
	f.Add("yes")
	f.Add("no")
	f.Add("1")
	f.Add("0")
	f.Add("")
	f.Add("invalid")
	f.Add("TRUE")
	f.Add("YES")
	f.Add("ON")
	f.Add("T")
	f.Add("Y")

	f.Fuzz(func(t *testing.T, input string) {
		// Should never panic
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = parseBoolGenerous(input)
	})
}

// FuzzParseTime tests time parsing with fuzz input
func FuzzParseTime(f *testing.F) {
	// Seed corpus with known formats
	f.Add("2024-01-15T10:30:00Z")
	f.Add("2024-01-15")
	f.Add("2024-01-15 10:30:00")
	f.Add("Mon, 15 Jan 2024 10:30:00 MST")
	f.Add("15 Jan 24 10:30 MST")
	f.Add("")
	f.Add("invalid")
	f.Add("2024-13-45") // invalid date
	f.Add("2024-01-15T25:00:00Z")
	f.Add("   ")

	f.Fuzz(func(t *testing.T, input string) {
		opts := defaultConfig()
		// Should never panic, even with invalid input
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_, _ = parseTime(input, opts)
	})
}

// FuzzExtractBracketKey tests bracket key extraction
func FuzzExtractBracketKey(f *testing.F) {
	// Seed corpus
	f.Add("metadata[key]", "metadata")
	f.Add("metadata[key.with.dots]", "metadata")
	f.Add("metadata[\"quoted\"]", "metadata")
	f.Add("metadata['single']", "metadata")
	f.Add("metadata[]", "metadata")
	f.Add("metadata[unclosed", "metadata")
	f.Add("metadata[a][b]", "metadata")
	f.Add("other[key]", "metadata")
	f.Add("", "metadata")
	f.Add("[key]", "")

	f.Fuzz(func(t *testing.T, fullKey, prefix string) {
		// Should never panic
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_ = extractBracketKey(fullKey, prefix)
	})
}

// FuzzQueryBinding tests query parameter binding with fuzz input
func FuzzQueryBinding(f *testing.F) {
	// Seed corpus with query strings
	f.Add("name=John&age=30")
	f.Add("name=&age=")
	f.Add("name=John%20Doe&age=30")
	f.Add("tags=go&tags=rust&tags=python")
	f.Add("invalid=%%%")
	f.Add("")
	f.Add("=value")
	f.Add("key=")
	f.Add("a=1&b=2&c=3&d=4&e=5")

	f.Fuzz(func(t *testing.T, queryString string) {
		type Params struct {
			Name string   `query:"name"`
			Age  int      `query:"age"`
			Tags []string `query:"tags"`
		}

		values, err := url.ParseQuery(queryString)
		if err != nil {
			return // Skip invalid query strings
		}

		var params Params
		// Should never panic, even with invalid input
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_ = Raw(NewQueryGetter(values), TagQuery, &params)
	})
}

// FuzzJSONBinding tests JSON binding with fuzz input
func FuzzJSONBinding(f *testing.F) {
	// Seed corpus with JSON strings
	f.Add(`{"name":"John","age":30}`)
	f.Add(`{"name":"","age":0}`)
	f.Add(`{}`)
	f.Add(`{"unknown":"field"}`)
	f.Add(`{"age":"not-a-number"}`)
	f.Add(`invalid json`)
	f.Add(``)
	f.Add(`null`)
	f.Add(`[]`)
	f.Add(`{"deeply":{"nested":{"value":42}}}`)

	f.Fuzz(func(t *testing.T, jsonInput string) {
		type User struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		var user User
		// Should never panic, even with invalid input
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_ = JSONTo([]byte(jsonInput), &user)
	})
}

// FuzzMapBinding tests map binding with various key patterns
func FuzzMapBinding(f *testing.F) {
	// Seed corpus with map key patterns
	f.Add("metadata.key1=value1&metadata.key2=value2")
	f.Add("metadata[key1]=value1&metadata[key2]=value2")
	f.Add("metadata[\"quoted.key\"]=value")
	f.Add("metadata['single.quoted']=value")
	f.Add("metadata[]=value")
	f.Add("")

	f.Fuzz(func(t *testing.T, queryString string) {
		type Params struct {
			Metadata map[string]string `query:"metadata"`
		}

		values, err := url.ParseQuery(queryString)
		if err != nil {
			return // Skip invalid query strings
		}

		var params Params
		// Should never panic
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_ = Raw(NewQueryGetter(values), TagQuery, &params)
	})
}

// FuzzNestedStructBinding tests nested struct binding
func FuzzNestedStructBinding(f *testing.F) {
	// Seed corpus
	f.Add("address.street=Main&address.city=NYC")
	f.Add("address.street=&address.city=")
	f.Add("address.deep.nested.value=test")
	f.Add("")

	f.Fuzz(func(t *testing.T, queryString string) {
		type Address struct {
			Street string `query:"street"`
			City   string `query:"city"`
		}
		type Params struct {
			Name    string  `query:"name"`
			Address Address `query:"address"`
		}

		values, err := url.ParseQuery(queryString)
		if err != nil {
			return
		}

		var params Params
		// Should never panic
		//nolint:errcheck // Fuzz test intentionally ignores errors; testing panic-safety only
		_ = Raw(NewQueryGetter(values), TagQuery, &params)
	})
}
