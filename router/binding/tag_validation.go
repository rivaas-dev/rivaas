//go:build !debug
// +build !debug

package binding

import "fmt"

// invalidTagf returns an error for invalid struct tags in production builds.
// In debug builds (with -tags debug), this panics instead.
func invalidTagf(format string, args ...any) error {
	return fmt.Errorf("invalid struct tag: "+format, args...)
}
