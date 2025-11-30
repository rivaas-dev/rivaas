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
	"encoding/xml"
	"fmt"
	"io"
)

// XML binds XML bytes to type T.
//
// Example:
//
//	user, err := binding.XML[CreateUserRequest](body)
//
//	// With options
//	user, err := binding.XML[CreateUserRequest](body,
//	    binding.WithRequired(),
//	)
//
// Errors:
//   - [ErrOutMustBePointer]: T is not a struct type
//   - [ErrRequiredField]: when [WithRequired] is used and a required field is missing
//   - [ErrMaxDepthExceeded]: struct nesting exceeds maximum depth
//   - [BindError]: field-level binding errors with detailed context
func XML[T any](body []byte, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	defer cfg.finish()
	if err := bindXMLBytesInternal(&result, body, cfg); err != nil {
		return result, err
	}
	return result, nil
}

// XMLReader binds XML from an io.Reader to type T.
//
// Example:
//
//	user, err := binding.XMLReader[CreateUserRequest](r.Body)
//
// Errors:
//   - [ErrOutMustBePointer]: T is not a struct type
//   - [ErrRequiredField]: when [WithRequired] is used and a required field is missing
//   - [ErrMaxDepthExceeded]: struct nesting exceeds maximum depth
//   - [BindError]: field-level binding errors with detailed context
func XMLReader[T any](r io.Reader, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	defer cfg.finish()
	if err := bindXMLReaderInternal(&result, r, cfg); err != nil {
		return result, err
	}
	return result, nil
}

// XMLTo binds XML bytes to out.
//
// Example:
//
//	var user CreateUserRequest
//	err := binding.XMLTo(body, &user)
func XMLTo(body []byte, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	defer cfg.finish()
	return bindXMLBytesInternal(out, body, cfg)
}

// XMLReaderTo binds XML from an io.Reader to out.
//
// Example:
//
//	var user CreateUserRequest
//	err := binding.XMLReaderTo(r.Body, &user)
func XMLReaderTo(r io.Reader, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	defer cfg.finish()
	return bindXMLReaderInternal(out, r, cfg)
}

// bindXMLReaderInternal binds XML from an io.Reader.
func bindXMLReaderInternal(out any, r io.Reader, cfg *config) error {
	decoder := xml.NewDecoder(r)
	if cfg.xmlStrict {
		decoder.Strict = true
	}

	if err := decoder.Decode(out); err != nil {
		cfg.trackError()
		return err
	}

	// Run validator if configured
	if cfg.validator != nil {
		if err := cfg.validator.Validate(out); err != nil {
			cfg.trackError()
			return &BindError{
				Field:  "",
				Source: SourceXML,
				Reason: fmt.Sprintf("validation failed: %v", err),
				Err:    err,
			}
		}
	}

	return nil
}

// bindXMLBytesInternal is the internal implementation for XML byte binding.
func bindXMLBytesInternal(out any, body []byte, cfg *config) error {
	if err := xml.Unmarshal(body, out); err != nil {
		cfg.trackError()
		return err
	}

	// Run validator if configured
	if cfg.validator != nil {
		if err := cfg.validator.Validate(out); err != nil {
			cfg.trackError()
			return &BindError{
				Field:  "",
				Source: SourceXML,
				Reason: fmt.Sprintf("validation failed: %v", err),
				Err:    err,
			}
		}
	}

	return nil
}
