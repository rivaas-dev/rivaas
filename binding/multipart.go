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

package binding

import (
	"fmt"
	"mime/multipart"
	"net/url"
	"strings"
)

// MultipartGetter implements both [ValueGetter] and [FileGetter] for multipart form data.
// It provides access to both form values and uploaded files from a multipart/form-data request.
//
// MultipartGetter is the core type for binding multipart forms that include file uploads
// and regular form fields (including nested JSON in form values).
//
// Example:
//
//	// In an HTTP handler:
//	r.ParseMultipartForm(32 << 20) // 32 MB max memory
//	getter := binding.NewMultipartGetter(r.MultipartForm)
//
//	// Access files
//	file, _ := getter.File("avatar")
//	file.Save("./uploads/" + file.Name)
//
//	// Access form values
//	name := getter.Get("name")
//
//	// Or bind to a struct
//	type Request struct {
//	    Avatar *File  `form:"avatar"`
//	    Name   string `form:"name"`
//	}
//	var req Request
//	binding.MultipartTo(r.MultipartForm, &req)
type MultipartGetter struct {
	values url.Values
	files  map[string][]*multipart.FileHeader
}

// NewMultipartGetter creates a MultipartGetter from a parsed multipart form.
// The form should be obtained from http.Request.MultipartForm after calling
// http.Request.ParseMultipartForm.
//
// Example:
//
//	if err := r.ParseMultipartForm(32 << 20); err != nil {
//	    return err
//	}
//	getter := binding.NewMultipartGetter(r.MultipartForm)
func NewMultipartGetter(form *multipart.Form) *MultipartGetter {
	return &MultipartGetter{
		values: form.Value,
		files:  form.File,
	}
}

// Get returns the first value for the key.
// Implements [ValueGetter].
func (m *MultipartGetter) Get(key string) string {
	return m.values.Get(key)
}

// GetAll returns all values for the key.
// It supports both repeated key patterns ("tags=go&tags=rust") and bracket notation
// ("tags[]=go&tags[]=rust").
// Implements [ValueGetter].
func (m *MultipartGetter) GetAll(key string) []string {
	// Try standard form first
	if vals := m.values[key]; len(vals) > 0 {
		return vals
	}
	// Try bracket notation
	return m.values[key+"[]"]
}

// Has returns whether the key exists in form values.
// Implements [ValueGetter].
func (m *MultipartGetter) Has(key string) bool {
	return m.values.Has(key) || m.values.Has(key+"[]")
}

// ApproxLen estimates the number of keys starting with the given prefix.
// It checks both dot notation (prefix.) and bracket notation (prefix[).
// This is used for map capacity estimation during binding.
func (m *MultipartGetter) ApproxLen(prefix string) int {
	count := 0
	prefixDot := prefix + "."
	prefixBracket := prefix + "["

	for key := range m.values {
		if strings.HasPrefix(key, prefixDot) || strings.HasPrefix(key, prefixBracket) {
			count++
		}
	}

	return count
}

// File returns the first uploaded file for the given field name.
// Returns [ErrFileNotFound] if no file exists for the field name.
// Implements [FileGetter].
//
// Example:
//
//	file, err := getter.File("avatar")
//	if err != nil {
//	    return err
//	}
//	file.Save("./uploads/" + file.Name)
func (m *MultipartGetter) File(name string) (*File, error) {
	headers, ok := m.files[name]
	if !ok || len(headers) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrFileNotFound, name)
	}

	return NewFile(headers[0]), nil
}

// Files returns all uploaded files for the given field name.
// Returns [ErrNoFilesFound] if no files exist for the field name.
// Implements [FileGetter].
//
// Example:
//
//	files, err := getter.Files("attachments")
//	if err != nil {
//	    return err
//	}
//	for _, f := range files {
//	    f.Save("./uploads/" + f.Name)
//	}
func (m *MultipartGetter) Files(name string) ([]*File, error) {
	headers, ok := m.files[name]
	if !ok || len(headers) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrNoFilesFound, name)
	}

	files := make([]*File, 0, len(headers))
	for _, header := range headers {
		files = append(files, NewFile(header))
	}

	return files, nil
}

// HasFile returns true if at least one file exists for the field name.
// Implements [FileGetter].
func (m *MultipartGetter) HasFile(name string) bool {
	headers, ok := m.files[name]
	return ok && len(headers) > 0
}
