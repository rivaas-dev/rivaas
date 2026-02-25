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

// Package diag provides diagnostic types for OpenAPI spec generation.
// This package contains warning types and codes that are used throughout
// the openapi package and its subpackages.
package diag

import (
	"fmt"
	"strings"
)

// Warning represents an informational, non-fatal issue during spec generation.
//
// Warnings are ADVISORY ONLY and never break execution.
// Use errors for issues that must stop the process.
//
// Common scenarios that produce warnings:
//   - Targeting OpenAPI 3.0 when using 3.1-only features (downlevel)
//   - Using deprecated API features
type Warning interface {
	// Code returns the warning identifier.
	// Compare with Warn* constants for type-safe checks.
	Code() WarningCode

	// Path returns the JSON pointer to the affected spec element.
	// Example: "#/webhooks", "#/info/summary"
	Path() string

	// Message returns a human-readable description.
	Message() string

	// Category returns the warning's category for grouping.
	Category() WarningCategory

	// String returns a formatted representation.
	String() string
}

// WarningCode identifies a specific warning type.
// Use the Warn* constants for type-safe comparisons.
type WarningCode string

// String returns the code as a string.
func (c WarningCode) String() string {
	return string(c)
}

// Category returns the code's category.
func (c WarningCode) Category() WarningCategory {
	switch {
	case len(c) >= 9 && c[:9] == "DOWNLEVEL":
		return CategoryDownlevel
	case len(c) >= 11 && c[:11] == "DEPRECATION":
		return CategoryDeprecation
	default:
		return CategoryUnknown
	}
}

// Downlevel Warnings (3.1 → 3.0 feature losses)
const (
	// WarnDownlevelWebhooks indicates webhooks were dropped (3.0 doesn't support them).
	WarnDownlevelWebhooks WarningCode = "DOWNLEVEL_WEBHOOKS"

	// WarnDownlevelInfoSummary indicates info.summary was dropped (3.0 doesn't support it).
	WarnDownlevelInfoSummary WarningCode = "DOWNLEVEL_INFO_SUMMARY"

	// WarnDownlevelLicenseIdentifier indicates license.identifier was dropped.
	WarnDownlevelLicenseIdentifier WarningCode = "DOWNLEVEL_LICENSE_IDENTIFIER"

	// WarnDownlevelMutualTLS indicates mutualTLS security scheme was dropped.
	WarnDownlevelMutualTLS WarningCode = "DOWNLEVEL_MUTUAL_TLS"

	// WarnDownlevelConstToEnum indicates JSON Schema const was converted to enum.
	WarnDownlevelConstToEnum WarningCode = "DOWNLEVEL_CONST_TO_ENUM"

	// WarnDownlevelConstToEnumConflict indicates const conflicted with existing enum.
	WarnDownlevelConstToEnumConflict WarningCode = "DOWNLEVEL_CONST_TO_ENUM_CONFLICT"

	// WarnDownlevelPathItems indicates $ref in pathItems was expanded.
	WarnDownlevelPathItems WarningCode = "DOWNLEVEL_PATH_ITEMS"

	// WarnDownlevelPatternProperties indicates patternProperties was dropped.
	WarnDownlevelPatternProperties WarningCode = "DOWNLEVEL_PATTERN_PROPERTIES"

	// WarnDownlevelUnevaluatedProperties indicates unevaluatedProperties was dropped.
	WarnDownlevelUnevaluatedProperties WarningCode = "DOWNLEVEL_UNEVALUATED_PROPERTIES"

	// WarnDownlevelContentEncoding indicates contentEncoding was dropped.
	WarnDownlevelContentEncoding WarningCode = "DOWNLEVEL_CONTENT_ENCODING"

	// WarnDownlevelContentMediaType indicates contentMediaType was dropped.
	WarnDownlevelContentMediaType WarningCode = "DOWNLEVEL_CONTENT_MEDIA_TYPE"

	// WarnDownlevelMultipleExamples indicates multiple examples were collapsed to one.
	WarnDownlevelMultipleExamples WarningCode = "DOWNLEVEL_MULTIPLE_EXAMPLES"
)

// Deprecation Warnings (using deprecated features)
const (
	// WarnDeprecationExampleSingular indicates using deprecated singular example field.
	WarnDeprecationExampleSingular WarningCode = "DEPRECATION_EXAMPLE_SINGULAR"
)

// WarningCategory groups related warning types.
type WarningCategory string

const (
	// CategoryUnknown for unrecognized warning codes.
	CategoryUnknown WarningCategory = "unknown"

	// CategoryDownlevel for 3.1 → 3.0 conversion feature losses.
	// The spec is still valid, just with reduced functionality.
	CategoryDownlevel WarningCategory = "downlevel"

	// CategoryDeprecation for deprecated feature usage.
	// The feature still works but is discouraged.
	CategoryDeprecation WarningCategory = "deprecation"
)

// String returns the category as a string.
func (c WarningCategory) String() string {
	return string(c)
}

// Warnings is a collection of Warning with helper methods.
// Warnings are informational and never break execution.
type Warnings []Warning

// Has returns true if any warning matches the given code.
func (ws Warnings) Has(code WarningCode) bool {
	for _, w := range ws {
		if w.Code() == code {
			return true
		}
	}
	return false
}

// HasAny returns true if any warning matches any given code.
func (ws Warnings) HasAny(codes ...WarningCode) bool {
	if len(codes) == 0 {
		return false
	}
	set := make(map[WarningCode]struct{}, len(codes))
	for _, c := range codes {
		set[c] = struct{}{}
	}
	for _, w := range ws {
		if _, ok := set[w.Code()]; ok {
			return true
		}
	}
	return false
}

// HasCategory returns true if any warning is in the given category.
func (ws Warnings) HasCategory(cat WarningCategory) bool {
	for _, w := range ws {
		if w.Category() == cat {
			return true
		}
	}
	return false
}

// Filter returns warnings matching the given codes.
func (ws Warnings) Filter(codes ...WarningCode) Warnings {
	if len(codes) == 0 {
		return nil
	}
	set := make(map[WarningCode]struct{}, len(codes))
	for _, c := range codes {
		set[c] = struct{}{}
	}
	result := make(Warnings, 0, len(ws))
	for _, w := range ws {
		if _, ok := set[w.Code()]; ok {
			result = append(result, w)
		}
	}
	return result
}

// FilterCategory returns warnings in the given category.
func (ws Warnings) FilterCategory(cat WarningCategory) Warnings {
	result := make(Warnings, 0, len(ws))
	for _, w := range ws {
		if w.Category() == cat {
			result = append(result, w)
		}
	}
	return result
}

// Exclude returns warnings NOT matching the given codes.
func (ws Warnings) Exclude(codes ...WarningCode) Warnings {
	if len(codes) == 0 {
		return ws
	}
	set := make(map[WarningCode]struct{}, len(codes))
	for _, c := range codes {
		set[c] = struct{}{}
	}
	result := make(Warnings, 0, len(ws))
	for _, w := range ws {
		if _, ok := set[w.Code()]; !ok {
			result = append(result, w)
		}
	}
	return result
}

// Each calls fn for each warning.
func (ws Warnings) Each(fn func(Warning)) {
	for _, w := range ws {
		fn(w)
	}
}

// Codes returns all unique warning codes in this collection.
func (ws Warnings) Codes() []WarningCode {
	seen := make(map[WarningCode]struct{}, len(ws))
	codes := make([]WarningCode, 0, len(ws))
	for _, w := range ws {
		if _, ok := seen[w.Code()]; !ok {
			seen[w.Code()] = struct{}{}
			codes = append(codes, w.Code())
		}
	}
	return codes
}

// Counts returns warning counts grouped by category.
func (ws Warnings) Counts() map[WarningCategory]int {
	counts := make(map[WarningCategory]int)
	for _, w := range ws {
		counts[w.Category()]++
	}
	return counts
}

// String returns a formatted string of all warnings.
func (ws Warnings) String() string {
	if len(ws) == 0 {
		return "no warnings"
	}
	var s strings.Builder
	fmt.Fprintf(&s, "%d warning(s):", len(ws))
	for i, w := range ws {
		fmt.Fprintf(&s, "\n  [%d] %s", i+1, w.String())
	}
	return s.String()
}

// warning is the concrete implementation of Warning interface.
type warning struct {
	code    WarningCode
	path    string
	message string
}

func (w *warning) Code() WarningCode {
	return w.code
}

func (w *warning) Path() string {
	return w.path
}

func (w *warning) Message() string {
	return w.message
}

func (w *warning) Category() WarningCategory {
	return w.code.Category()
}

func (w *warning) String() string {
	return fmt.Sprintf("[%s] %s: %s", w.code.Category(), w.code, w.message)
}

// NewWarning creates a new Warning instance.
// This is the primary way to create warnings from internal packages.
func NewWarning(code WarningCode, path, message string) Warning {
	return &warning{
		code:    code,
		path:    path,
		message: message,
	}
}
