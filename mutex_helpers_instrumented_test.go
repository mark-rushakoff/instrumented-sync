// +build isync

package sync_test

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/mark-rushakoff/instrumented-sync"
)

// drainMutex continually reads from all mutex-related channels.
// It's intended to be called once from the test package.
func drainMutex() {
	go func() {
		for range sync.MutexRegistrationOut {
		}
	}()
	go func() {
		for range sync.MutexStatOut {
		}
	}()
	go func() {
		for range sync.MutexDeregistrationOut {
		}
	}()
}
