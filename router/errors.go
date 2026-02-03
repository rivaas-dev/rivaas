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

var (
	// ErrContextResponseNil indicates that the context response is nil.
	ErrContextResponseNil = errors.New("context response is nil")

	// ErrContentTypeNotAllowed indicates that the content type is not allowed.
	ErrContentTypeNotAllowed = errors.New("content type not allowed")

	// ErrResponseWriterNotHijacker indicates that ResponseWriter does not implement the http.Hijacker interface.
	ErrResponseWriterNotHijacker = errors.New("responseWriter does not implement http.Hijacker")

	// ErrBloomFilterSizeZero indicates that the bloom filter size must be greater than zero.
	ErrBloomFilterSizeZero = errors.New("bloom filter size must be non-zero")

	// ErrBloomHashFunctionsInvalid indicates that the number of bloom hash functions must be positive.
	ErrBloomHashFunctionsInvalid = errors.New("bloom hash functions must be positive")

	// ErrVersioningConfigInvalid indicates that the versioning configuration is invalid.
	ErrVersioningConfigInvalid = errors.New("versioning configuration invalid")

	// ErrServerTimeoutInvalid indicates that the server timeout value must be positive.
	ErrServerTimeoutInvalid = errors.New("server timeout must be positive")

	// ErrRoutesNotFrozen indicates that the routes have not been frozen yet.
	ErrRoutesNotFrozen = errors.New("routes not frozen yet")

	// ErrRouteNotFound indicates that the specified route could not be found.
	ErrRouteNotFound = errors.New("route not found")

	// ErrMissingRouteParameter indicates that a required parameter for the route is missing.
	ErrMissingRouteParameter = errors.New("missing required parameter")

	// ErrMultipleJSONValues indicates that the request body must contain only a single JSON value.
	ErrMultipleJSONValues = errors.New("request body must contain a single JSON value")

	// ErrExpectedJSONArray indicates that a JSON array was expected.
	ErrExpectedJSONArray = errors.New("expected a JSON array")

	// ErrArrayExceedsMax indicates that the JSON array exceeds the maximum allowed number of items.
	ErrArrayExceedsMax = errors.New("array exceeds maximum items")

	// ErrCannotValidateNilValue indicates that a nil value cannot be validated.
	ErrCannotValidateNilValue = errors.New("cannot validate nil value")

	// ErrCannotValidateInvalidValue indicates that an invalid value cannot be validated.
	ErrCannotValidateInvalidValue = errors.New("cannot validate invalid value")

	// ErrUnknownValidationStrategy indicates that an unknown validation strategy was encountered.
	ErrUnknownValidationStrategy = errors.New("unknown validation strategy")

	// ErrCannotRegisterValidators indicates that validators cannot be registered after the first use.
	ErrCannotRegisterValidators = errors.New("cannot register validators after first use")

	// ErrQueryInvalidInteger indicates that a query parameter contains an invalid integer.
	ErrQueryInvalidInteger = errors.New("query: invalid integer")
)
