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

// Test helpers for all formatter tests

type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

type testErrorWithCode struct {
	message string
	code    string
}

func (e *testErrorWithCode) Error() string {
	return e.message
}

func (e *testErrorWithCode) Code() string {
	return e.code
}

type testErrorWithStatus struct {
	message string
	status  int
}

func (e *testErrorWithStatus) Error() string {
	return e.message
}

func (e *testErrorWithStatus) HTTPStatus() int {
	return e.status
}

type testErrorFull struct {
	message string
	code    string
	status  int
}

func (e *testErrorFull) Error() string {
	return e.message
}

func (e *testErrorFull) Code() string {
	return e.code
}

func (e *testErrorFull) HTTPStatus() int {
	return e.status
}

type testErrorWithDetails struct {
	message string
	details map[string]any
}

func (e *testErrorWithDetails) Error() string {
	return e.message
}

func (e *testErrorWithDetails) Details() any {
	return e.details
}

type testErrorWithDetailsSlice struct {
	message string
	details []map[string]any
}

func (e *testErrorWithDetailsSlice) Error() string {
	return e.message
}

func (e *testErrorWithDetailsSlice) Details() any {
	return e.details
}
