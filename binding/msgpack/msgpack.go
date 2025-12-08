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

// Package msgpack provides MessagePack binding support for the binding package.
//
// This package extends rivaas.dev/binding with MessagePack serialization support,
// using github.com/vmihailenco/msgpack/v5 for parsing.
//
// Example:
//
//	type Message struct {
//	    ID      int64  `msgpack:"id"`
//	    Content string `msgpack:"content"`
//	}
//
//	msg, err := msgpack.MsgPack[Message](body)
//	if err != nil {
//	    // handle error
//	}
package msgpack

import (
	"bytes"
	"fmt"
	"io"

	"github.com/vmihailenco/msgpack/v5"

	"rivaas.dev/binding"
)

// Option configures MessagePack binding behavior.
type Option func(*config)

// config holds MessagePack-specific binding configuration.
type config struct {
	validator      binding.Validator
	useJSONTag     bool // Use json tag for field names instead of msgpack
	disallowUnknow bool // Disallow unknown fields
}

// WithValidator integrates external validation.
// The validator is called after successful binding.
func WithValidator(v binding.Validator) Option {
	return func(c *config) {
		c.validator = v
	}
}

// WithJSONTag enables using JSON struct tags for field names.
// By default, msgpack struct tags are used.
func WithJSONTag() Option {
	return func(c *config) {
		c.useJSONTag = true
	}
}

// WithDisallowUnknown enables strict mode that returns an error
// if the MessagePack data contains fields not in the struct.
func WithDisallowUnknown() Option {
	return func(c *config) {
		c.disallowUnknow = true
	}
}

func applyOptions(opts []Option) *config {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// MsgPack binds MessagePack bytes to type T.
//
// Example:
//
//	msg, err := msgpack.MsgPack[Message](body)
//
//	// With options
//	msg, err := msgpack.MsgPack[Message](body, msgpack.WithJSONTag())
func MsgPack[T any](body []byte, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	if err := bindMsgPackBytes(&result, body, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// MsgPackReader binds MessagePack from an io.Reader to type T.
//
// Example:
//
//	msg, err := msgpack.MsgPackReader[Message](r.Body)
func MsgPackReader[T any](r io.Reader, opts ...Option) (T, error) {
	var result T
	cfg := applyOptions(opts)
	if err := bindMsgPackReader(&result, r, cfg); err != nil {
		return result, err
	}

	return result, nil
}

// MsgPackTo binds MessagePack bytes to out.
//
// Example:
//
//	var msg Message
//	err := msgpack.MsgPackTo(body, &msg)
func MsgPackTo(body []byte, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	return bindMsgPackBytes(out, body, cfg)
}

// MsgPackReaderTo binds MessagePack from an io.Reader to out.
//
// Example:
//
//	var msg Message
//	err := msgpack.MsgPackReaderTo(r.Body, &msg)
func MsgPackReaderTo(r io.Reader, out any, opts ...Option) error {
	cfg := applyOptions(opts)
	return bindMsgPackReader(out, r, cfg)
}

func bindMsgPackBytes(out any, body []byte, cfg *config) error {
	// If we need custom options, use decoder; otherwise use simple unmarshal
	var err error
	if cfg.useJSONTag || cfg.disallowUnknow {
		err = decodeWithOptions(bytes.NewReader(body), out, cfg)
	} else {
		err = msgpack.Unmarshal(body, out)
	}
	if err != nil {
		return err
	}

	return runValidator(out, cfg)
}

// decodeWithOptions creates a decoder with custom options and decodes.
func decodeWithOptions(r io.Reader, out any, cfg *config) error {
	dec := msgpack.NewDecoder(r)
	if cfg.useJSONTag {
		dec.SetCustomStructTag("json")
	}
	if cfg.disallowUnknow {
		dec.DisallowUnknownFields(true)
	}

	return dec.Decode(out)
}

// runValidator runs the configured validator if present.
func runValidator(out any, cfg *config) error {
	if cfg.validator == nil {
		return nil
	}

	if err := cfg.validator.Validate(out); err != nil {
		return &binding.BindError{
			Field:  "",
			Source: binding.SourceMsgPack,
			Reason: fmt.Sprintf("validation failed: %v", err),
			Err:    err,
		}
	}

	return nil
}

func bindMsgPackReader(out any, r io.Reader, cfg *config) error {
	if err := decodeWithOptions(r, out, cfg); err != nil {
		return err
	}

	return runValidator(out, cfg)
}

// sourceGetter is a marker type for MessagePack body source.
type sourceGetter struct {
	body   []byte
	cfg    *config
	reader io.Reader
}

func (s *sourceGetter) Get(key string) string      { return "" }
func (s *sourceGetter) GetAll(key string) []string { return nil }
func (s *sourceGetter) Has(key string) bool        { return false }

// FromMsgPack returns a binding.Option that specifies MessagePack body as a binding source.
// This can be used with binding.Bind for multi-source binding.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    binding.FromQuery(r.URL.Query()),
//	    msgpack.FromMsgPack(body),
//	)
func FromMsgPack(body []byte, opts ...Option) binding.Option {
	cfg := applyOptions(opts)
	return binding.FromGetter(&sourceGetter{body: body, cfg: cfg}, binding.TagMsgPack)
}

// FromMsgPackReader returns a binding.Option that specifies MessagePack from io.Reader as a binding source.
//
// Example:
//
//	req, err := binding.Bind[Request](
//	    msgpack.FromMsgPackReader(r.Body),
//	)
func FromMsgPackReader(r io.Reader, opts ...Option) binding.Option {
	cfg := applyOptions(opts)
	return binding.FromGetter(&sourceGetter{reader: r, cfg: cfg}, binding.TagMsgPack)
}
