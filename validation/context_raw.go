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

package validation

import "context"

// ctxKey is a private type for context keys to avoid collisions with other packages.
type ctxKey int

const (
	ctxKeyRawJSON ctxKey = iota
)

// InjectRawJSONCtx injects raw JSON bytes into context for [StrategyJSONSchema] validation.
//
// InjectRawJSONCtx is an internal API used by request binding frameworks. The raw bytes are used
// directly for schema validation instead of re-encoding the Go struct back to JSON.
//
// Do not call InjectRawJSONCtx directly in application code unless you are implementing a custom
// binding framework.
//
// Parameters:
//   - ctx: The parent context
//   - raw: Raw JSON bytes from the request body
func InjectRawJSONCtx(ctx context.Context, raw []byte) context.Context {
	return context.WithValue(ctx, ctxKeyRawJSON, raw)
}

// ExtractRawJSONCtx retrieves raw JSON bytes from context if present.
// ExtractRawJSONCtx returns (nil, false) if the context does not contain raw JSON
// (i.e., [InjectRawJSONCtx] was not called).
func ExtractRawJSONCtx(ctx context.Context) ([]byte, bool) {
	raw, ok := ctx.Value(ctxKeyRawJSON).([]byte)
	return raw, ok
}
