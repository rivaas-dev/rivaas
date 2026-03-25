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

// JSON binds JSON bytes to type T.
//
// Example:
//
//	user, err := binding.JSON[CreateUserRequest](body)
//
//	// With options
//	user, err := binding.JSON[CreateUserRequest](body,
//	    binding.WithUnknownFields(binding.UnknownError),
//	)
//
// Errors:
//   - [ErrOutMustBePointer]: T is not a struct type
//   - [ErrMaxDepthExceeded]: struct nesting exceeds maximum depth
//   - [UnknownFieldError]: when [WithUnknownFields] is [UnknownError] and unknown fields are present
//   - [BindError]: field-level binding errors with detailed context
//   - [MultiError]: when [WithAllErrors] is used and multiple errors occur
func JSON[T any](body []byte, opts ...Option) (T, error) {
	var result T
	cfg, err := applyOptions(opts)
	if err != nil {
		return result, err
	}
	defer cfg.finish()
	if err = bindJSONBytesInternal(&result, body, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// JSONReader binds JSON from an io.Reader to type T.
//
// Example:
//
//	user, err := binding.JSONReader[CreateUserRequest](r.Body)
//
// Errors:
//   - [ErrOutMustBePointer]: T is not a struct type
//   - [ErrMaxDepthExceeded]: struct nesting exceeds maximum depth
//   - [UnknownFieldError]: when [WithUnknownFields] is [UnknownError] and unknown fields are present
//   - [BindError]: field-level binding errors with detailed context
func JSONReader[T any](r io.Reader, opts ...Option) (T, error) {
	var result T
	cfg, err := applyOptions(opts)
	if err != nil {
		return result, err
	}
	defer cfg.finish()
	if err = bindJSONReaderInternal(&result, r, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// JSONTo binds JSON bytes to out.
//
// Example:
//
//	var user CreateUserRequest
//	err := binding.JSONTo(body, &user)
func JSONTo(body []byte, out any, opts ...Option) error {
	cfg, err := applyOptions(opts)
	if err != nil {
		return err
	}
	defer cfg.finish()

	return bindJSONBytesInternal(out, body, cfg)
}

// JSONReaderTo binds JSON from an io.Reader to out.
//
// Example:
//
//	var user CreateUserRequest
//	err := binding.JSONReaderTo(r.Body, &user)
func JSONReaderTo(r io.Reader, out any, opts ...Option) error {
	cfg, err := applyOptions(opts)
	if err != nil {
		return err
	}
	defer cfg.finish()

	return bindJSONReaderInternal(out, r, cfg)
}

// bindJSONReaderInternal binds JSON from an io.Reader.
func bindJSONReaderInternal(out any, r io.Reader, cfg *config) error {
	// For Warn/Error policies we need the raw bytes to walk the structure.
	// We also need buffered bytes when the target struct has defaults, so we
	// can build the presence map after decoding.
	needsBuffer := cfg.unknownFields == UnknownWarn ||
		cfg.unknownFields == UnknownError ||
		jsonStructHasDefaults(out)

	if needsBuffer {
		body, err := io.ReadAll(r)
		if err != nil {
			cfg.trackError()
			return err
		}

		return bindJSONBytesInternal(out, body, cfg)
	}

	// Fast path: no unknown-field detection, no defaults — stream decode.
	decoder := json.NewDecoder(r)
	if cfg.jsonUseNumber {
		decoder.UseNumber()
	}
	if err := decoder.Decode(out); err != nil {
		cfg.trackError()
		return err
	}

	return nil
}

// bindJSONBytesInternal is the internal implementation for JSON byte binding.
func bindJSONBytesInternal(out any, body []byte, cfg *config) error {
	switch cfg.unknownFields {
	case UnknownError:
		// Use standard decoder with DisallowUnknownFields
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if cfg.jsonUseNumber {
			decoder.UseNumber()
		}

		if err := decoder.Decode(out); err != nil {
			cfg.trackError()

			// Check if error is due to unknown field
			if strings.Contains(err.Error(), "unknown field") {
				fieldName := extractUnknownFieldName(err.Error())
				cfg.unknowns = append(cfg.unknowns, fieldName)

				return &UnknownFieldError{Fields: []string{fieldName}}
			}

			return err
		}

	case UnknownWarn:
		// Two-pass: detect unknowns, then decode.
		// Use context.Background() because the binding public API doesn't expose context.
		// TODO: Consider adding context support to binding functions for cancellation during
		// expensive reflection operations in high-load scenarios.
		if err := bindJSONWithWarnings(context.Background(), out, body, cfg); err != nil {
			return err
		}

	default: // UnknownIgnore
		decoder := json.NewDecoder(bytes.NewReader(body))
		if cfg.jsonUseNumber {
			decoder.UseNumber()
		}
		if err := decoder.Decode(out); err != nil {
			cfg.trackError()
			return err
		}
	}

	// Post-decode: apply default:"..." values to fields absent from the payload.
	// Only parse the raw map when the struct actually has defaults.
	if jsonStructHasDefaults(out) {
		var raw map[string]json.RawMessage
		if unmarshalErr := json.Unmarshal(body, &raw); unmarshalErr == nil {
			if defaultsErr := applyJSONDefaults(out, raw, cfg); defaultsErr != nil {
				return defaultsErr
			}
		}
	}

	return nil
}

// bindJSONWithWarnings detects unknown fields at all nesting levels and warns.
func bindJSONWithWarnings(ctx context.Context, out any, body []byte, cfg *config) error {
	// First: decode into generic map to get full structure
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		cfg.trackError()
		return err
	}

	// Check context before expensive operations
	if err := ctx.Err(); err != nil {
		cfg.trackError()
		return err
	}

	// Build trie of allowed field paths from struct type
	t := reflect.TypeOf(out)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	trie := newJSONFieldTrie(t, TagJSON)

	// Walk JSON structure and detect unknowns (recursive)
	if err := walkJSONRawMessage(json.RawMessage(body), trie, nil, func(path string) {
		cfg.unknowns = append(cfg.unknowns, path)
	}); err != nil {
		cfg.trackError()
		return err
	}

	// Second: decode into target struct (using original bytes for efficiency)
	decoder := json.NewDecoder(bytes.NewReader(body))
	if cfg.jsonUseNumber {
		decoder.UseNumber()
	}
	if err := decoder.Decode(out); err != nil {
		cfg.trackError()
		return err
	}

	return nil
}

// applyJSONDefaults walks a decoded struct and applies default:"..." values
// to fields whose JSON key was absent from the payload. Presence is determined
// by the raw map produced from json.Unmarshal into map[string]json.RawMessage.
// For nested structs whose parent key IS present, the function recurses into
// the nested raw value.
func applyJSONDefaults(out any, raw map[string]json.RawMessage, cfg *config) error {
	v := reflect.ValueOf(out)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	return applyJSONDefaultsToValue(v, raw, cfg)
}

// applyJSONDefaultsToValue is the recursive core that operates on reflect.Value.
func applyJSONDefaultsToValue(v reflect.Value, raw map[string]json.RawMessage, cfg *config) error {
	info := getStructInfo(v.Type(), TagJSON)
	if !info.hasDefaults {
		return nil
	}

	for _, field := range info.fields {
		jsonKey := field.tagName
		if jsonKey == "" || jsonKey == "-" {
			continue
		}

		rawVal, present := raw[jsonKey]

		// Recurse into nested structs that are present in the payload
		if field.isStruct && present {
			fv := v.FieldByIndex(field.index)
			if !fv.CanSet() {
				continue
			}

			var nested map[string]json.RawMessage
			if unmarshalErr := json.Unmarshal(rawVal, &nested); unmarshalErr == nil {
				if recurseErr := applyJSONDefaultsToValue(fv, nested, cfg); recurseErr != nil {
					return recurseErr
				}
			}

			continue
		}

		if field.defaultValue == "" {
			continue
		}

		if present {
			continue
		}

		fv := v.FieldByIndex(field.index)
		if !fv.CanSet() {
			continue
		}

		if applied := applyTypedDefault(v, field); applied {
			continue
		}

		// Fallback: runtime conversion for slices and scalars
		if field.isSlice {
			values := splitAndTrimCSV(field.defaultValue)
			if err := setSliceField(fv, values, cfg); err != nil {
				return &BindError{
					Field:  field.name,
					Source: SourceJSON,
					Value:  field.defaultValue,
					Type:   fv.Type(),
					Err:    err,
				}
			}
		} else {
			if err := setField(fv, field.defaultValue, field.isPtr, cfg); err != nil {
				return &BindError{
					Field:  field.name,
					Source: SourceJSON,
					Value:  field.defaultValue,
					Type:   fv.Type(),
					Err:    err,
				}
			}
		}
	}

	return nil
}

// jsonStructHasDefaults checks whether the target type has any default tags
// when parsed for JSON binding. Returns false for non-struct types.
func jsonStructHasDefaults(out any) bool {
	t := reflect.TypeOf(out)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return false
	}

	info := getStructInfo(t, TagJSON)

	return info.hasDefaults
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
