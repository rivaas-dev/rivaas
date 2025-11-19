package validation

import "context"

// ctxKey is a private type for context keys to avoid collisions.
type ctxKey int

const (
	ctxKeyRawJSON ctxKey = iota
)

// InjectRawJSONCtx injects raw JSON bytes into context for schema validation optimization.
//
// This is an internal optimization API used by request binding frameworks to avoid
// re-marshaling data during JSON Schema validation. The raw bytes are used directly
// for schema validation instead of encoding the Go struct back to JSON.
//
// Do not call this directly in application code unless you are implementing a custom
// binding framework.
func InjectRawJSONCtx(ctx context.Context, raw []byte) context.Context {
	return context.WithValue(ctx, ctxKeyRawJSON, raw)
}

// ExtractRawJSONCtx retrieves raw JSON bytes from context if present.
// Returns (nil, false) if not found.
func ExtractRawJSONCtx(ctx context.Context) ([]byte, bool) {
	raw, ok := ctx.Value(ctxKeyRawJSON).([]byte)
	return raw, ok
}
