//go:build !darwin

package main

import "errors"

// cloneFile is only implemented on macOS; elsewhere callers fall back to a
// regular copy.
func cloneFile(_, _ string) error {
	return errors.ErrUnsupported
}
