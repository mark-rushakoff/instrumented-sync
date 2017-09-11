// +build !isync

package sync_test

// mutexReset is intended to be called as `defer mutexReset(x)()`
// to reinitialize the package state at the beginning of any mutex test.
func mutexReset(drain bool) func() {
	return func() {}
}
