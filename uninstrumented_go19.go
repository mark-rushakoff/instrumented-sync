// +build go1.9,!isync

package sync

import(
	"sync"
)

// Alias the types that would otherwise be instrumented if the instrumented-sync build flag was set.
type Mutex = sync.Mutex
