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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
)

// BindJSON binds JSON request body to a struct.
// It reads from the provided io.Reader and decodes JSON into the target struct.
// Unknown field handling is controlled by UnknownFieldPolicy in options.
//
// Parameters:
//   - out: Pointer to struct that will receive bound values
//   - r: io.Reader containing JSON data
//   - opts: Optional configuration options
//
// Returns an error if JSON decoding fails or validation errors occur.
func BindJSON(out any, r io.Reader, opts ...Option) error {
	return BindJSONWithContext(context.Background(), out, r, opts...)
}

// BindJSONWithContext binds JSON with context support for cancellation.
// It respects context cancellation during JSON decoding.
//
// Parameters:
//   - ctx: Context for cancellation
//   - out: Pointer to struct that will receive bound values
//   - r: io.Reader containing JSON data
//   - opts: Optional configuration options
//
// Returns an error if context is cancelled, JSON decoding fails, or validation errors occur.
func BindJSONWithContext(ctx context.Context, out any, r io.Reader, opts ...Option) error {
	options := applyOptions(opts)
	defer options.finish()

	// For Warn/Error policies, we need the raw bytes to walk the structure
	if options.UnknownFields == UnknownWarn || options.UnknownFields == UnknownError {
		// Read body into memory
		body, err := io.ReadAll(r)
		if err != nil {
			options.trackError()
			return err
		}
		return bindJSONBytesWithContext(ctx, out, body, options)
	}

	// No unknown field detection needed
	decoder := json.NewDecoder(r)
	if options.JSONUseNumber {
		decoder.UseNumber()
	}
	return decodeWithContext(ctx, decoder, out, options)
}

// BindJSONBytes binds JSON from a byte slice.
// Use this when the request body is already available as bytes, such as when
// it has been cached by the router.
//
// Parameters:
//   - out: Pointer to struct that will receive bound values
//   - body: JSON data as byte slice
//   - opts: Optional configuration options
//
// Returns an error if JSON decoding fails or validation errors occur.
func BindJSONBytes(out any, body []byte, opts ...Option) error {
	return BindJSONBytesWithContext(context.Background(), out, body, opts...)
}

// BindJSONBytesWithContext binds JSON bytes with context support for cancellation.
// It respects context cancellation during JSON decoding.
//
// Parameters:
//   - ctx: Context for cancellation
//   - out: Pointer to struct that will receive bound values
//   - body: JSON data as byte slice
//   - opts: Optional configuration options
//
// Returns an error if context is cancelled, JSON decoding fails, or validation errors occur.
func BindJSONBytesWithContext(ctx context.Context, out any, body []byte, opts ...Option) error {
	options := applyOptions(opts)
	defer options.finish()
	return bindJSONBytesWithContext(ctx, out, body, options)
}

// bindJSONBytesWithContext is the internal implementation.
func bindJSONBytesWithContext(ctx context.Context, out any, body []byte, opts *Options) error {
	// Check context before heavy work
	if err := ctx.Err(); err != nil {
		opts.trackError()
		return err
	}

	switch opts.UnknownFields {
	case UnknownError:
		// Use standard decoder with DisallowUnknownFields
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if opts.JSONUseNumber {
			decoder.UseNumber()
		}

		if err := decoder.Decode(out); err != nil {
			opts.trackError()

			// Check if error is due to unknown field
			if strings.Contains(err.Error(), "unknown field") {
				fieldName := extractUnknownFieldName(err.Error())
				if opts.Events.UnknownField != nil {
					opts.Events.UnknownField(fieldName)
				}
				return &UnknownFieldError{Fields: []string{fieldName}}
			}

			return err
		}
		return nil

	case UnknownWarn:
		// Two-pass: detect unknowns, then decode
		return bindJSONWithWarnings(ctx, out, body, opts)

	default: // UnknownIgnore
		decoder := json.NewDecoder(bytes.NewReader(body))
		if opts.JSONUseNumber {
			decoder.UseNumber()
		}
		if err := decoder.Decode(out); err != nil {
			opts.trackError()
			return err
		}
		return nil
	}
}

// bindJSONWithWarnings detects unknown fields at all nesting levels and warns.
func bindJSONWithWarnings(ctx context.Context, out any, body []byte, opts *Options) error {
	// First: decode into generic map to get full structure
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		opts.trackError()
		return err
	}

	// Check context before expensive operations
	if err := ctx.Err(); err != nil {
		opts.trackError()
		return err
	}

	// Build trie of allowed field paths from struct type
	t := reflect.TypeOf(out)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	trie := newJSONFieldTrie(t, TagJSON)

	// Walk JSON structure and detect unknowns (recursive)
	unknowns := []string{}
	if err := walkJSONRawMessage(json.RawMessage(body), trie, nil, func(path string) {
		unknowns = append(unknowns, path)
		evtFlags := opts.eventFlags()
		if evtFlags.hasUnknownField {
			opts.Events.UnknownField(path)
		}
	}); err != nil {
		opts.trackError()
		return err
	}

	// Second: decode into target struct (using original bytes for efficiency)
	decoder := json.NewDecoder(bytes.NewReader(body))
	if opts.JSONUseNumber {
		decoder.UseNumber()
	}
	if err := decoder.Decode(out); err != nil {
		opts.trackError()
		return err
	}

	// Unknowns are logged via events but don't fail
	_ = unknowns

	return nil
}

// BindJSONStrict is a convenience wrapper for strict JSON decoding.
// It is equivalent to BindJSON with UnknownFieldPolicy set to UnknownError,
// which returns an error when unknown fields are encountered.
//
// Parameters:
//   - out: Pointer to struct that will receive bound values
//   - r: io.Reader containing JSON data
//   - opts: Optional configuration options (UnknownFieldPolicy is overridden)
//
// Returns an error if unknown fields are present, JSON decoding fails, or validation errors occur.
func BindJSONStrict(out any, r io.Reader, opts ...Option) error {
	opts = append(opts, WithUnknownFieldPolicy(UnknownError))
	return BindJSON(out, r, opts...)
}

// BindJSONStrictBytes is a convenience wrapper for strict JSON decoding from bytes.
// It is equivalent to BindJSONBytes with UnknownFieldPolicy set to UnknownError.
//
// Parameters:
//   - out: Pointer to struct that will receive bound values
//   - body: JSON data as byte slice
//   - opts: Optional configuration options (UnknownFieldPolicy is overridden)
//
// Returns an error if unknown fields are present, JSON decoding fails, or validation errors occur.
func BindJSONStrictBytes(out any, body []byte, opts ...Option) error {
	opts = append(opts, WithUnknownFieldPolicy(UnknownError))
	return BindJSONBytes(out, body, opts...)
}

// decodeWithContext performs JSON decoding with periodic context checks.
func decodeWithContext(ctx context.Context, decoder *json.Decoder, out any, opts *Options) error {
	// Create a channel to signal decode completion
	type result struct {
		err error
	}

	done := make(chan result, 1)

	go func() {
		done <- result{err: decoder.Decode(out)}
	}()

	select {
	case <-ctx.Done():
		opts.trackError()
		return ctx.Err()
	case res := <-done:
		if res.err != nil {
			opts.trackError()
		}
		return res.err
	}
}

// extractUnknownFieldName parses json.Decoder error to extract field name.
func extractUnknownFieldName(errMsg string) string {
	// Example error: "json: unknown field \"extra_field\""
	start := strings.Index(errMsg, "\"")
	if start == -1 {
		return ""
	}
	end := strings.Index(errMsg[start+1:], "\"")
	if end == -1 {
		return ""
	}
	return errMsg[start+1 : start+1+end]
}

// BindJSONInto binds JSON into a new instance of type T and returns it.
// It is a convenience wrapper around BindJSONBytes that eliminates the need to
// create and pass a pointer manually.
//
// Example:
//
//	result, err := BindJSONInto[UserRequest](body)
//
// Parameters:
//   - body: JSON data as byte slice
//   - opts: Optional configuration options
//
// Returns the bound value of type T and an error if binding fails.
func BindJSONInto[T any](body []byte, opts ...Option) (T, error) {
	var result T
	err := BindJSONBytes(&result, body, opts...)
	return result, err
}
