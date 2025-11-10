//go:build debug
// +build debug

package binding

import "fmt"

// invalidTagf panics for invalid struct tags in debug builds.
// In production builds (without -tags debug), this returns an error instead.
func invalidTagf(format string, args ...any) error {
	panic(fmt.Sprintf("invalid struct tag: "+format, args...))
}
