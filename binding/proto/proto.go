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

// Package proto provides Protocol Buffers binding support for the binding package.
//
// This package extends rivaas.dev/binding with Protocol Buffers serialization support,
// using google.golang.org/protobuf for parsing.
//
// Example:
//
//	// Assuming you have generated proto code:
//	// message User {
//	//     string name = 1;
//	//     int32 age = 2;
//	// }
//
//	user, err := proto.Proto[*pb.User](body)
//	if err != nil {
//	    // handle error
//	}
package proto

import (
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
	"rivaas.dev/binding"
)

// Message is an alias for proto.Message to simplify imports.
type Message = proto.Message

// Option configures Protocol Buffers binding behavior.
type Option func(*config)

// config holds Proto-specific binding configuration.
type config struct {
	validator      binding.Validator
	unmarshalOpts  proto.UnmarshalOptions
	allowPartial   bool
	discardUnknown bool
	recursionLimit int
}

// WithValidator integrates external validation.
// The validator is called after successful binding.
func WithValidator(v binding.Validator) Option {
	return func(c *config) {
		c.validator = v
	}
}

// WithAllowPartial allows messages that have missing required fields to unmarshal
// without returning an error.
func WithAllowPartial() Option {
	return func(c *config) {
		c.allowPartial = true
	}
}

// WithDiscardUnknown specifies whether to ignore unknown fields when unmarshaling.
func WithDiscardUnknown() Option {
	return func(c *config) {
		c.discardUnknown = true
	}
}

// WithRecursionLimit sets the maximum recursion depth for unmarshaling.
// The default limit is 10000.
func WithRecursionLimit(limit int) Option {
	return func(c *config) {
		c.recursionLimit = limit
	}
}

func applyOptions(opts []Option) *config {
	cfg := &config{
		recursionLimit: 10000, // default
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

func (c *config) toUnmarshalOptions() proto.UnmarshalOptions {
	return proto.UnmarshalOptions{
		AllowPartial:   c.allowPartial,
		DiscardUnknown: c.discardUnknown,
		RecursionLimit: c.recursionLimit,
	}
}

// Proto binds Protocol Buffers bytes to type T.
// T must implement proto.Message.
//
// Example:
//
//	user, err := proto.Proto[*pb.User](body)
//
//	// With options
//	user, err := proto.Proto[*pb.User](body, proto.WithDiscardUnknown())
func Proto[T Message](body []byte, opts ...Option) (T, error) {
	var result T

	// Create a new instance of T
	// Since T is a pointer to a proto message, we need to get the element type
	result = result.ProtoReflect().New().Interface().(T)

	cfg := applyOptions(opts)
	if err := bindProtoBytes(result, body, cfg); err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

// ProtoReader binds Protocol Buffers from an io.Reader to type T.
// T must implement proto.Message.
//
// Example:
//
//	user, err := proto.ProtoReader[*pb.User](r.Body)
func ProtoReader[T Message](r io.Reader, opts ...Option) (T, error) {
	var result T

	// Create a new instance of T
	result = result.ProtoReflect().New().Interface().(T)

	cfg := applyOptions(opts)
	if err := bindProtoReader(result, r, cfg); err != nil {
		var zero T
		return zero, err
	}
	return result, nil
}

// ProtoTo binds Protocol Buffers bytes to out.
// out must implement proto.Message.
//
// Example:
//
//	var user pb.User
//	err := proto.ProtoTo(body, &user)
func ProtoTo(body []byte, out Message, opts ...Option) error {
	cfg := applyOptions(opts)
	return bindProtoBytes(out, body, cfg)
}

// ProtoReaderTo binds Protocol Buffers from an io.Reader to out.
// out must implement proto.Message.
//
// Example:
//
//	var user pb.User
//	err := proto.ProtoReaderTo(r.Body, &user)
func ProtoReaderTo(r io.Reader, out Message, opts ...Option) error {
	cfg := applyOptions(opts)
	return bindProtoReader(out, r, cfg)
}

func bindProtoBytes(out Message, body []byte, cfg *config) error {
	unmarshalOpts := cfg.toUnmarshalOptions()

	if err := unmarshalOpts.Unmarshal(body, out); err != nil {
		return err
	}

	// Run validator if configured
	if cfg.validator != nil {
		if err := cfg.validator.Validate(out); err != nil {
			return &binding.BindError{
				Field:  "",
				Source: binding.SourceProto,
				Reason: fmt.Sprintf("validation failed: %v", err),
				Err:    err,
			}
		}
	}

	return nil
}

func bindProtoReader(out Message, r io.Reader, cfg *config) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	return bindProtoBytes(out, body, cfg)
}

// sourceGetter is a marker type for Proto body source.
type sourceGetter struct {
	body   []byte
	cfg    *config
	reader io.Reader
}

func (s *sourceGetter) Get(key string) string      { return "" }
func (s *sourceGetter) GetAll(key string) []string { return nil }
func (s *sourceGetter) Has(key string) bool        { return false }

// FromProto returns a binding.Option that specifies Protocol Buffers body as a binding source.
// This can be used with binding.Bind for multi-source binding.
//
// Note: When using FromProto, you need to ensure the target struct implements proto.Message.
//
// Example:
//
//	req, err := binding.Bind[*pb.Request](
//	    binding.FromQuery(r.URL.Query()),
//	    proto.FromProto(body),
//	)
func FromProto(body []byte, opts ...Option) binding.Option {
	cfg := applyOptions(opts)
	return binding.FromGetter(&sourceGetter{body: body, cfg: cfg}, binding.TagProto)
}

// FromProtoReader returns a binding.Option that specifies Protocol Buffers from io.Reader as a binding source.
//
// Example:
//
//	req, err := binding.Bind[*pb.Request](
//	    proto.FromProtoReader(r.Body),
//	)
func FromProtoReader(r io.Reader, opts ...Option) binding.Option {
	cfg := applyOptions(opts)
	return binding.FromGetter(&sourceGetter{reader: r, cfg: cfg}, binding.TagProto)
}
