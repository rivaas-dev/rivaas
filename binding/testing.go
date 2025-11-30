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
	"net/http"
	"net/url"
	"testing"
)

// TestBinder creates a Binder configured for testing.
// It uses sensible defaults that are appropriate for test scenarios.
//
// Example:
//
//	func TestMyFeature(t *testing.T) {
//	    binder := binding.TestBinder(t)
//	    // use binder in test
//	}
func TestBinder(t *testing.T, opts ...Option) *Binder {
	t.Helper()

	defaultOpts := []Option{
		WithMaxDepth(DefaultMaxDepth),
		WithMaxMapSize(DefaultMaxMapSize),
		WithMaxSliceLen(DefaultMaxSliceLen),
	}

	// Append user-provided options (they will override defaults)
	allOpts := append(defaultOpts, opts...)

	binder, err := New(allOpts...)
	if err != nil {
		t.Fatalf("TestBinder: failed to create binder: %v", err)
	}
	return binder
}

// TestQueryGetter creates a QueryGetter from key-value pairs for testing.
// It provides a more convenient way to create query parameters in tests.
//
// Example:
//
//	getter := binding.TestQueryGetter(t, "name", "John", "age", "30")
func TestQueryGetter(t *testing.T, pairs ...string) ValueGetter {
	t.Helper()

	if len(pairs)%2 != 0 {
		t.Fatalf("TestQueryGetter: pairs must be key-value pairs, got odd number of arguments")
	}

	values := url.Values{}
	for i := 0; i < len(pairs); i += 2 {
		values.Set(pairs[i], pairs[i+1])
	}
	return NewQueryGetter(values)
}

// TestQueryGetterMulti creates a QueryGetter that supports multiple values per key.
// Useful for testing slice bindings.
//
// Example:
//
//	getter := binding.TestQueryGetterMulti(t, map[string][]string{
//	    "tags": {"go", "rust", "python"},
//	    "page": {"1"},
//	})
func TestQueryGetterMulti(t *testing.T, values map[string][]string) ValueGetter {
	t.Helper()
	return NewQueryGetter(url.Values(values))
}

// TestFormGetter creates a FormGetter from key-value pairs for testing.
//
// Example:
//
//	getter := binding.TestFormGetter(t, "username", "testuser", "password", "secret")
func TestFormGetter(t *testing.T, pairs ...string) ValueGetter {
	t.Helper()

	if len(pairs)%2 != 0 {
		t.Fatalf("TestFormGetter: pairs must be key-value pairs, got odd number of arguments")
	}

	values := url.Values{}
	for i := 0; i < len(pairs); i += 2 {
		values.Set(pairs[i], pairs[i+1])
	}
	return NewFormGetter(values)
}

// TestPathGetter creates a PathGetter from key-value pairs for testing.
//
// Example:
//
//	getter := binding.TestPathGetter(t, "user_id", "123", "slug", "hello-world")
func TestPathGetter(t *testing.T, pairs ...string) ValueGetter {
	t.Helper()

	if len(pairs)%2 != 0 {
		t.Fatalf("TestPathGetter: pairs must be key-value pairs, got odd number of arguments")
	}

	params := make(map[string]string)
	for i := 0; i < len(pairs); i += 2 {
		params[pairs[i]] = pairs[i+1]
	}
	return NewPathGetter(params)
}

// TestHeaderGetter creates a HeaderGetter from key-value pairs for testing.
//
// Example:
//
//	getter := binding.TestHeaderGetter(t, "Authorization", "Bearer token", "X-Request-ID", "123")
func TestHeaderGetter(t *testing.T, pairs ...string) ValueGetter {
	t.Helper()

	if len(pairs)%2 != 0 {
		t.Fatalf("TestHeaderGetter: pairs must be key-value pairs, got odd number of arguments")
	}

	header := http.Header{}
	for i := 0; i < len(pairs); i += 2 {
		header.Set(pairs[i], pairs[i+1])
	}
	return NewHeaderGetter(header)
}

// TestCookieGetter creates a CookieGetter from key-value pairs for testing.
//
// Example:
//
//	getter := binding.TestCookieGetter(t, "session_id", "abc123", "theme", "dark")
func TestCookieGetter(t *testing.T, pairs ...string) ValueGetter {
	t.Helper()

	if len(pairs)%2 != 0 {
		t.Fatalf("TestCookieGetter: pairs must be key-value pairs, got odd number of arguments")
	}

	cookies := make([]*http.Cookie, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		cookies = append(cookies, &http.Cookie{
			Name:  pairs[i],
			Value: pairs[i+1],
		})
	}
	return NewCookieGetter(cookies)
}

// AssertBindError checks if an error is a BindError with the expected field name.
// Returns the BindError if found, fails the test otherwise.
//
// Example:
//
//	err := binding.Raw(getter, binding.TagQuery, &params)
//	bindErr := binding.AssertBindError(t, err, "Age")
//	assert.Equal(t, binding.SourceQuery, bindErr.Source)
func AssertBindError(t *testing.T, err error, expectedField string) *BindError {
	t.Helper()

	if err == nil {
		t.Fatalf("AssertBindError: expected BindError for field %q, got nil", expectedField)
	}

	bindErr, ok := err.(*BindError)
	if !ok {
		t.Fatalf("AssertBindError: expected *BindError, got %T: %v", err, err)
	}

	if bindErr.Field != expectedField {
		t.Fatalf("AssertBindError: expected field %q, got %q", expectedField, bindErr.Field)
	}

	return bindErr
}

// AssertNoBindError asserts that the error is nil.
// Provides a cleaner failure message than require.NoError for binding contexts.
//
// Example:
//
//	err := binding.Raw(getter, binding.TagQuery, &params)
//	binding.AssertNoBindError(t, err)
func AssertNoBindError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}

	if bindErr, ok := err.(*BindError); ok {
		t.Fatalf("AssertNoBindError: binding failed for field %q (%s): %v",
			bindErr.Field, bindErr.Source, bindErr.Error())
	}

	t.Fatalf("AssertNoBindError: unexpected error: %v", err)
}

// MustBind is a test helper that binds and fails the test if binding fails.
// It's useful when binding must succeed for the test to proceed.
//
// Example:
//
//	params := binding.MustBind[SearchParams](t, getter, binding.TagQuery)
func MustBind[T any](t *testing.T, getter ValueGetter, tag string, opts ...Option) T {
	t.Helper()

	result, err := RawInto[T](getter, tag, opts...)
	if err != nil {
		if bindErr, ok := err.(*BindError); ok {
			t.Fatalf("MustBind[%T]: binding failed for field %q (%s): %v",
				result, bindErr.Field, bindErr.Source, bindErr.Error())
		}
		t.Fatalf("MustBind[%T]: binding failed: %v", result, err)
	}
	return result
}

// MustBindJSON is a test helper for JSON binding that fails if binding fails.
//
// Example:
//
//	user := binding.MustBindJSON[User](t, `{"name":"John","age":30}`)
func MustBindJSON[T any](t *testing.T, jsonData string, opts ...Option) T {
	t.Helper()

	result, err := JSON[T]([]byte(jsonData), opts...)
	if err != nil {
		t.Fatalf("MustBindJSON[%T]: binding failed: %v", result, err)
	}
	return result
}

// MustBindQuery is a test helper for query binding that fails if binding fails.
//
// Example:
//
//	params := binding.MustBindQuery[SearchParams](t, url.Values{"q": {"golang"}})
func MustBindQuery[T any](t *testing.T, values url.Values, opts ...Option) T {
	t.Helper()

	result, err := Query[T](values, opts...)
	if err != nil {
		if bindErr, ok := err.(*BindError); ok {
			t.Fatalf("MustBindQuery[%T]: binding failed for field %q (%s): %v",
				result, bindErr.Field, bindErr.Source, bindErr.Error())
		}
		t.Fatalf("MustBindQuery[%T]: binding failed: %v", result, err)
	}
	return result
}

// MustBindForm is a test helper for form binding that fails if binding fails.
//
// Example:
//
//	data := binding.MustBindForm[FormData](t, url.Values{"username": {"test"}})
func MustBindForm[T any](t *testing.T, values url.Values, opts ...Option) T {
	t.Helper()

	result, err := Form[T](values, opts...)
	if err != nil {
		if bindErr, ok := err.(*BindError); ok {
			t.Fatalf("MustBindForm[%T]: binding failed for field %q (%s): %v",
				result, bindErr.Field, bindErr.Source, bindErr.Error())
		}
		t.Fatalf("MustBindForm[%T]: binding failed: %v", result, err)
	}
	return result
}

// TestValidator is a mock validator for testing validation integration.
type TestValidator struct {
	ValidateFunc func(v any) error
}

// Validate implements the Validator interface.
func (tv *TestValidator) Validate(v any) error {
	if tv.ValidateFunc != nil {
		return tv.ValidateFunc(v)
	}
	return nil
}

// NewTestValidator creates a TestValidator with the given validation function.
//
// Example:
//
//	validator := binding.NewTestValidator(func(v any) error {
//	    user, ok := v.(*User)
//	    if !ok {
//	        return nil
//	    }
//	    if user.Age < 0 {
//	        return errors.New("age must be non-negative")
//	    }
//	    return nil
//	})
func NewTestValidator(fn func(v any) error) *TestValidator {
	return &TestValidator{ValidateFunc: fn}
}

// AlwaysFailValidator returns a validator that always returns an error.
// Useful for testing error handling paths.
//
// Example:
//
//	validator := binding.AlwaysFailValidator("validation failed")
func AlwaysFailValidator(msg string) *TestValidator {
	return &TestValidator{
		ValidateFunc: func(v any) error {
			return &BindError{
				Field:  "",
				Source: SourceUnknown,
				Reason: msg,
			}
		},
	}
}

// NeverFailValidator returns a validator that never returns an error.
//
// Example:
//
//	validator := binding.NeverFailValidator()
func NeverFailValidator() *TestValidator {
	return &TestValidator{
		ValidateFunc: func(v any) error {
			return nil
		},
	}
}
