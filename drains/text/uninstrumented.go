// +build !isync

package text

import "io"

// Drain is a no-op when building without sync instrumentation.
func Drain(w io.Writer) {}
