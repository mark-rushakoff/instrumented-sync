// +build isync

package sync_test

import "github.com/mark-rushakoff/instrumented-sync"

// mutexReset reassigns the public variables related to mutexes.
// It is intended to be called as `defer mutexReset(x)()` at the beginning of any mutex test.
func mutexReset(drain bool) func() {
	sync.MutexStatOut = make(chan sync.MutexStat, sync.MutexStatOutSize)
	sync.MutexRegistrationOut = make(chan sync.MutexRegistration, sync.MutexRegistrationOutSize)

	if drain {
		go func() {
			for range sync.MutexStatOut {
			}
		}()
		go func() {
			for range sync.MutexRegistrationOut {
			}
		}()
		go func() {
			for range sync.MutexDeregistrationOut {
			}
		}()
	}

	return func() {
		// TODO: this causes a race where the channel is closed before the final send completes.
		// May be able to observe values to determine when all mutexes are no longer used.
		close(sync.MutexStatOut)
		close(sync.MutexRegistrationOut)
	}
}
