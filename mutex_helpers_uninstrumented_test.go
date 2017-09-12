// +build !isync

package sync_test

// drainMutex continually reads from all mutex-related channels.
// It's intended to be called once from the test package.
//
// Without the isync build tag, this is a no-op.
func drainMutex() {}
