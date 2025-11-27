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

package version

import "errors"

// Static errors for version configuration validation.
// These errors should be wrapped with fmt.Errorf and %w when context is needed.
var (
	// Detection strategy errors
	ErrEmptyPathPattern          = errors.New("path pattern cannot be empty")
	ErrMissingVersionPlaceholder = errors.New("pattern must contain {version} placeholder")
	ErrEmptyHeaderName           = errors.New("header name cannot be empty")
	ErrEmptyQueryParam           = errors.New("query parameter name cannot be empty")
	ErrEmptyAcceptPattern        = errors.New("accept pattern cannot be empty")
	ErrNilCustomDetector         = errors.New("custom detector function cannot be nil")

	// Configuration errors
	ErrEmptyDefaultVersion = errors.New("default version cannot be empty")
	ErrNoValidVersions     = errors.New("at least one valid version is required")
	ErrEmptyVersionEntry   = errors.New("version cannot be empty")
	ErrDefaultRequired     = errors.New("default version is required")
)
