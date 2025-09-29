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

package export

// Warning represents a warning generated during spec projection.
//
// Warnings are generated when 3.1-only features are used with a 3.0 target,
// or when features need to be down-leveled. The Code field provides a
// machine-readable identifier for filtering or suppression.
type Warning struct {
	// Code is a machine-readable warning code (e.g., "DOWNLEVEL_UNEVALUATED_PROPERTIES").
	Code string

	// Path is a JSON Pointer to the location in the spec where the warning occurred.
	// Empty string indicates a top-level issue.
	Path string

	// Message is a human-readable description of the warning.
	Message string
}

// Warning codes for common down-leveling scenarios.
const (
	// DOWNLEVEL_CONST_TO_ENUM indicates const was converted to enum for 3.0 compatibility.
	DOWNLEVEL_CONST_TO_ENUM = "DOWNLEVEL_CONST_TO_ENUM"

	// DOWNLEVEL_CONST_TO_ENUM_CONFLICT indicates const conflicted with existing enum.
	DOWNLEVEL_CONST_TO_ENUM_CONFLICT = "DOWNLEVEL_CONST_TO_ENUM_CONFLICT"

	// DOWNLEVEL_UNEVALUATED_PROPERTIES indicates unevaluatedProperties was dropped.
	DOWNLEVEL_UNEVALUATED_PROPERTIES = "DOWNLEVEL_UNEVALUATED_PROPERTIES"

	// DOWNLEVEL_PATTERN_PROPERTIES indicates patternProperties may not be fully supported.
	DOWNLEVEL_PATTERN_PROPERTIES = "DOWNLEVEL_PATTERN_PROPERTIES"

	// DOWNLEVEL_MULTIPLE_EXAMPLES indicates multiple examples were reduced to one.
	DOWNLEVEL_MULTIPLE_EXAMPLES = "DOWNLEVEL_MULTIPLE_EXAMPLES"

	// DOWNLEVEL_WEBHOOKS indicates webhooks were dropped (3.1-only feature).
	DOWNLEVEL_WEBHOOKS = "DOWNLEVEL_WEBHOOKS"

	// DOWNLEVEL_LICENSE_IDENTIFIER indicates license identifier was dropped (3.1-only feature).
	DOWNLEVEL_LICENSE_IDENTIFIER = "DOWNLEVEL_LICENSE_IDENTIFIER"

	// DOWNLEVEL_INFO_SUMMARY indicates info.summary was dropped (3.1-only feature).
	DOWNLEVEL_INFO_SUMMARY = "DOWNLEVEL_INFO_SUMMARY"

	// DOWNLEVEL_MUTUAL_TLS indicates mutualTLS security type was dropped (3.1-only feature).
	DOWNLEVEL_MUTUAL_TLS = "DOWNLEVEL_MUTUAL_TLS"

	// SERVER_VARIABLE_EMPTY_ENUM indicates server variable enum array is empty (invalid in 3.1).
	SERVER_VARIABLE_EMPTY_ENUM = "SERVER_VARIABLE_EMPTY_ENUM"

	// SERVER_VARIABLE_DEFAULT_NOT_IN_ENUM indicates server variable default is not in enum (invalid in 3.1).
	SERVER_VARIABLE_DEFAULT_NOT_IN_ENUM = "SERVER_VARIABLE_DEFAULT_NOT_IN_ENUM"
)
