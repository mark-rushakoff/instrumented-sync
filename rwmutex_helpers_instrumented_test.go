// +build isync

package sync_test

import "github.com/mark-rushakoff/instrumented-sync"

// drainRWMutex continually reads from all RWMutex-related channels.
// It's intended to be called once from the test package.
func drainRWMutex() {
	go func() {
		for range sync.RWMutexRegistrationOut {
		}
	}()
	go func() {
		for range sync.RWMutexStatOut {
		}
	}()
	go func() {
		for range sync.RWMutexDeregistrationOut {
		}
	}()
}
