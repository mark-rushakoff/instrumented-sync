// +build !isync

package sync_test

// drainRWMutex continually reads from all RWMutex-related channels.
// It's intended to be called once from the test package.
//
// Without the isync build tag, this is a no-op.
func drainRWMutex() {}
