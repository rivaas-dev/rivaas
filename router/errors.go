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

import "errors"

// Static errors for better error handling and testing.
// These errors should be wrapped with fmt.Errorf and %w when context is needed.
var (
	// Context errors
	ErrContextResponseNil    = errors.New("context response is nil")
	ErrContentTypeNotAllowed = errors.New("content type not allowed")

	// Request errors
	ErrFileNotFound = errors.New("file not found")
	ErrNoFilesFound = errors.New("no files found for key")

	// Router errors
	ErrResponseWriterNotHijacker = errors.New("responseWriter does not implement http.Hijacker")

	// Router configuration errors
	ErrBloomFilterSizeZero       = errors.New("bloom filter size must be non-zero")
	ErrBloomHashFunctionsInvalid = errors.New("bloom hash functions must be positive")

	// Route errors
	ErrRoutesNotFrozen       = errors.New("routes not frozen yet")
	ErrRouteNotFound         = errors.New("route not found")
	ErrMissingRouteParameter = errors.New("missing required parameter")

	// JSON parsing errors
	ErrMultipleJSONValues = errors.New("request body must contain a single JSON value")
	ErrExpectedJSONArray  = errors.New("expected a JSON array")
	ErrArrayExceedsMax    = errors.New("array exceeds maximum items")

	// Validation errors
	ErrCannotValidateNilValue     = errors.New("cannot validate nil value")
	ErrCannotValidateInvalidValue = errors.New("cannot validate invalid value")
	ErrUnknownValidationStrategy  = errors.New("unknown validation strategy")
	ErrCannotRegisterValidators   = errors.New("cannot register validators after first use")

	// Test errors (for test files)
	ErrInvalidUUIDFormat    = errors.New("invalid UUID format: must be 36 characters")
	ErrReadError            = errors.New("read error")
	ErrBindingFailed        = errors.New("binding failed")
	ErrCookieNotFound       = errors.New("cookie not found")
	ErrUserIDRequired       = errors.New("user ID is required")
	ErrPageParameterInvalid = errors.New("page parameter is invalid")
	ErrInvalidType          = errors.New("invalid type")
	ErrCustomNameRequired   = errors.New("custom: name is required")
	ErrGenericValidation    = errors.New("generic validation error")
	ErrOuterError           = errors.New("outer error")
	ErrGenericError         = errors.New("generic error")
	ErrQueryInvalidInteger  = errors.New("query: invalid integer")
)
