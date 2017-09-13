// +build go1.9

// Alias the stdlib sync types we haven't (yet?) instrumented,
// regardless of whether the instrumented-sync build flag is being used.

package sync

import (
	"sync"
)

type Cond = sync.Cond
type Locker = sync.Locker
type Map = sync.Map
type Once = sync.Once
type Pool = sync.Pool
type WaitGroup = sync.WaitGroup
