package binding

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strings"
)

// BindJSON binds JSON request body to a struct with UnknownFieldPolicy support.
func BindJSON(out any, r io.Reader, opts ...Option) error {
	return BindJSONWithContext(context.Background(), out, r, opts...)
}

// BindJSONWithContext binds JSON with context support for cancellation.
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

	// Fast path: no unknown field detection needed
	decoder := json.NewDecoder(r)
	if options.JSONUseNumber {
		decoder.UseNumber()
	}
	return decodeWithContext(ctx, decoder, out, options)
}

// BindJSONBytes binds JSON from byte slice (avoids extra copies when body is cached).
func BindJSONBytes(out any, body []byte, opts ...Option) error {
	return BindJSONBytesWithContext(context.Background(), out, body, opts...)
}

// BindJSONBytesWithContext binds JSON bytes with context support.
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
	walkJSONRawMessage(json.RawMessage(body), trie, nil, func(path string) {
		unknowns = append(unknowns, path)
		evtFlags := opts.eventFlags()
		if evtFlags.hasUnknownField {
			opts.Events.UnknownField(path)
		}
	})

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
// Equivalent to BindJSON with UnknownFieldPolicy = UnknownError.
func BindJSONStrict(out any, r io.Reader, opts ...Option) error {
	opts = append(opts, WithUnknownFieldPolicy(UnknownError))
	return BindJSON(out, r, opts...)
}

// BindJSONStrictBytes is a convenience wrapper for strict JSON decoding from bytes.
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

// BindJSONInto is generic sugar for JSON binding.
func BindJSONInto[T any](body []byte, opts ...Option) (T, error) {
	var result T
	err := BindJSONBytes(&result, body, opts...)
	return result, err
}
