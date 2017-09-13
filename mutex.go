// +build isync

package sync

import (
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

// The buffer size of the mutex stat output.
// You could vary this with a linker setting.
var MutexStatOutSize = 4096

// Channel for mutex stats.
// There is no default reader; if you don't read from the channel,
// it will eventually deadlock your program.
var MutexStatOut = make(chan MutexStat, MutexStatOutSize)

// MutexStat is a summary of details about a particular Mutex.
type MutexStat struct {
	ID      uint64
	Waiting uint32
	Held    uint32
}

// The buffer size of the mutex registration output.
// You could vary this with a linker setting.
var MutexRegistrationOutSize = 128

// Channel for mutex registrations.
// There is no default reader; if you don't read from the channel,
// it will eventually deadlock your program.
var MutexRegistrationOut = make(chan MutexRegistration, MutexRegistrationOutSize)

// The buffer size of the mutex registration output.
// You could vary this with a linker setting.
var MutexDeregistrationOutSize = 128

// Channel for mutex deregistrations, where IDs are sent once the mutex is garbage collected.
// There is no default reader; if you don't read from the channel,
// it will eventually deadlock your program.
var MutexDeregistrationOut = make(chan uint64, MutexRegistrationOutSize)

// MutexRegistration is sent on the MutexRegistrationOut channel the first time Lock is called on a Mutex.
type MutexRegistration struct {
	ID    uint64
	Stack []byte
}

// Mutex wraps a standard library sync.Mutex for instrumentation.
type Mutex struct {
	m sync.Mutex

	// How many attempted locks are in progress.
	// Heap allocated so we can refer to the value in a goroutine without keeping the Mutex alive.
	waiting *uint32

	// How many are holding the lock - must always be 0 or 1.
	// Heap allocated so we can refer to the value in a goroutine without keeping the Mutex alive.
	held *uint32

	// Has the mutex ever been used?
	// Okay to be stack allocated.
	inUse uint32

	// We need something heap-allocated to ensure we can use runtime.SetFinalizer.
	// If a value is stack-allocated, runtime.SetFinalizer will panic.
	// Therefore, we will set a finalizer on this byte pointer that we will always allocate with new,
	// the first time Lock is called.
	live *byte
}

func (m *Mutex) Lock() {
	if atomic.CompareAndSwapUint32(&m.inUse, 0, 1) {
		m.onFirstUse()
	}

	// Ensure that we can use m.waiting, which we're about to increment, for the rest of this function.
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&m.waiting))) == nil {
		atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&m.waiting)), nil, unsafe.Pointer(new(uint32)))
	}

	// Indicate that a goroutine is waiting to acquire the lock.
	atomic.AddUint32(m.waiting, 1)

	m.m.Lock()

	// About to use held; it's possible that onFirstUse is on another goroutine and hasn't finished yet,
	// so ensure it's initialized here.
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&m.held))) == nil {
		atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&m.held)), nil, unsafe.Pointer(new(uint32)))
	}
	// There can only be one hold on the lock.
	// Indicate the lock is held, now that we've acquired the inner lock.
	atomic.StoreUint32(m.held, 1)

	// Decrement the number waiting to hold the lock.
	atomic.AddUint32(m.waiting, ^uint32(0))
}

func (m *Mutex) Unlock() {
	// Before we unlock the inner mutex, indicate there is no longer a hold on the lock.
	// This means there is a potential race of seeing m.held == 0 between unlock and immediate relock.
	atomic.StoreUint32(m.held, 0)
	m.m.Unlock()
}

var mutexCounter uint64

// onFirstUse ensures all values are initialized correctly, sets up a reporting goroutine,
// and registers a finalizer to close the reporting goroutine.
func (m *Mutex) onFirstUse() {
	id := atomic.AddUint64(&mutexCounter, 1)
	MutexRegistrationOut <- MutexRegistration{ID: id, Stack: debug.Stack()}

	done := make(chan struct{})

	// Ensure waiting and held are initialized properly.
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&m.waiting))) == nil {
		atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&m.waiting)), nil, unsafe.Pointer(new(uint32)))
	}
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&m.held))) == nil {
		atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&m.held)), nil, unsafe.Pointer(new(uint32)))
	}
	// Reference the stat fields directly so that we do not hold a reference to m.
	go reportMutex(done, m.waiting, m.held, id)

	// Set the finalizer on a value we are sure is heap-allocated.
	// See the comment for Mutex.live.
	m.live = new(byte)
	runtime.SetFinalizer(m.live, func(b *byte) {
		close(done)
		MutexDeregistrationOut <- id
	})
}

// reportMutexInterval determines how frequently an instrumented mutex should report its stats.
// For now, it's hardcoded. It may be configurable later.
func reportMutexInterval() time.Duration {
	return 25 * time.Millisecond
}

func reportMutex(done <-chan struct{}, waiting, held *uint32, id uint64) {
	t := time.NewTimer(reportMutexInterval())
	for {
		select {
		case <-done:
			t.Stop()
			return
		case <-t.C:
			MutexStatOut <- MutexStat{
				ID:      id,
				Waiting: atomic.LoadUint32(waiting),
				Held:    atomic.LoadUint32(held),
			}
			t.Reset(reportMutexInterval())
		}
	}
}
