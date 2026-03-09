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

package router

import (
	"bufio"
	"net"
	"net/http"
)

// WrittenChecker is implemented by response writers that track whether headers have been written.
// Used by context and response helpers to avoid duplicate WriteHeader calls.
type WrittenChecker interface {
	Written() bool
}

// ResponseWriterWrapper wraps http.ResponseWriter to capture status code, size, and written state.
// It also prevents "superfluous response.WriteHeader call" errors.
//
// Not safe for concurrent use; one instance per request, same as http.ResponseWriter.
type ResponseWriterWrapper struct {
	http.ResponseWriter

	statusCode int
	size       int64
	written    bool
}

// NewResponseWriterWrapper returns a new ResponseWriterWrapper that wraps w.
func NewResponseWriterWrapper(w http.ResponseWriter) *ResponseWriterWrapper {
	return &ResponseWriterWrapper{ResponseWriter: w}
}

// WriteHeader captures the status code and prevents duplicate calls.
func (rw *ResponseWriterWrapper) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

// Write captures the response size and marks as written.
func (rw *ResponseWriterWrapper) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += int64(n)

	return n, err
}

// StatusCode returns the HTTP status code.
func (rw *ResponseWriterWrapper) StatusCode() int {
	if rw.statusCode == 0 {
		return http.StatusOK
	}

	return rw.statusCode
}

// Size returns the response size in bytes.
func (rw *ResponseWriterWrapper) Size() int64 {
	return rw.size
}

// Written returns true if headers have been written.
func (rw *ResponseWriterWrapper) Written() bool {
	return rw.written
}

// AddSize adds n to the tracked response size. Used by wrappers that implement io.ReaderFrom.
func (rw *ResponseWriterWrapper) AddSize(n int64) {
	rw.size += n
}

// MarkWritten marks headers as written and sets status to 200 if not yet set. Used by wrappers that implement io.ReaderFrom.
func (rw *ResponseWriterWrapper) MarkWritten() {
	if !rw.written {
		rw.written = true
		if rw.statusCode == 0 {
			rw.statusCode = http.StatusOK
		}
	}
}

// Compile-time check that ResponseWriterWrapper implements ResponseInfo and WrittenChecker.
var (
	_ ResponseInfo   = (*ResponseWriterWrapper)(nil)
	_ WrittenChecker = (*ResponseWriterWrapper)(nil)
)

// Hijack implements http.Hijacker interface.
func (rw *ResponseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}

	return nil, nil, ErrResponseWriterNotHijacker
}

// Flush implements http.Flusher interface.
func (rw *ResponseWriterWrapper) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
