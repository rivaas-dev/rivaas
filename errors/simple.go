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

package errors

import (
	"errors"
	"net/http"
)

// Simple formats errors as simple JSON objects.
// Format: {"error": "message", "details": {...}, "code": "..."}
type Simple struct {
	// StatusResolver determines HTTP status from error.
	// If nil, uses ErrorType interface or defaults to 500.
	StatusResolver func(err error) int
}

// Format converts an error into a simple JSON response.
func (f *Simple) Format(req *http.Request, err error) Response {
	status := f.determineStatus(err)

	body := map[string]any{
		"error": err.Error(),
	}

	// Add details if available
	if detailed, ok := err.(ErrorDetails); ok {
		body["details"] = detailed.Details()
	}

	// Add code if available
	if coded, ok := err.(ErrorCode); ok {
		body["code"] = coded.Code()
	}

	return Response{
		Status:      status,
		ContentType: "application/json; charset=utf-8",
		Body:        body,
	}
}

func (f *Simple) determineStatus(err error) int {
	if f.StatusResolver != nil {
		return f.StatusResolver(err)
	}

	var typed ErrorType
	if errors.As(err, &typed) {
		return typed.HTTPStatus()
	}

	return http.StatusInternalServerError
}
