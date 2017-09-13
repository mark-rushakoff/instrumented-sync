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
var RWMutexStatOutSize = 4096

// Channel for RWMutex stats.
// There is no default reader; if you don't read from the channel,
// it will eventually deadlock your program.
var RWMutexStatOut = make(chan RWMutexStat, RWMutexStatOutSize)

// RWMutexStat is a summary of details about a particular RWMutex.
type RWMutexStat struct {
	ID                uint64
	Waiting, RWaiting uint32
	Held, RHeld       uint32
}

// The buffer size of the mutex registration output.
// You could vary this with a linker setting.
var RWMutexRegistrationOutSize = 128

// Channel for mutex registrations.
// There is no default reader; if you don't read from the channel,
// it will eventually deadlock your program.
var RWMutexRegistrationOut = make(chan RWMutexRegistration, RWMutexRegistrationOutSize)

// The buffer size of the mutex registration output.
// You could vary this with a linker setting.
var RWMutexDeregistrationOutSize = 128

// Channel for mutex deregistrations, where IDs are sent once the mutex is garbage collected.
// There is no default reader; if you don't read from the channel,
// it will eventually deadlock your program.
var RWMutexDeregistrationOut = make(chan uint64, RWMutexRegistrationOutSize)

// RWMutexRegistration is sent on the RWMutexRegistrationOut channel the first time Lock is called on a Mutex.
type RWMutexRegistration struct {
	ID    uint64
	Stack []byte
}

// RWMutex wraps a standard library sync.RWMutex for instrumentation.
type RWMutex struct {
	m sync.RWMutex

	// How many attempted RLocks and Locks are in progress.
	rwaiting, waiting *uint32

	// How many are holding the lock.
	rheld, held *uint32

	// Whether the mutex has ever been Locked or RLocked.
	inUse uint32

	// Heap-allocated value to use with runtime.SetFinalizer.
	live *byte
}

func (m *RWMutex) Lock() {
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

func (m *RWMutex) Unlock() {
	// Before we unlock the inner mutex, indicate there is no longer a hold on the lock.
	// This means there is a potential race of seeing m.held == 0 between unlock and immediate relock.
	atomic.StoreUint32(m.held, 0)
	m.m.Unlock()
}

func (m *RWMutex) RLock() {
	if atomic.CompareAndSwapUint32(&m.inUse, 0, 1) {
		m.onFirstUse()
	}

	// Ensure that we can use m.rwaiting, which we're about to increment, for the rest of this function.
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&m.rwaiting))) == nil {
		atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&m.rwaiting)), nil, unsafe.Pointer(new(uint32)))
	}

	// Indicate that a goroutine is waiting to acquire the lock.
	atomic.AddUint32(m.rwaiting, 1)

	m.m.RLock()

	// About to use rheld; it's possible that onFirstUse is on another goroutine and hasn't finished yet,
	// so ensure it's initialized here.
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&m.rheld))) == nil {
		atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&m.rheld)), nil, unsafe.Pointer(new(uint32)))
	}
	// Indicate the rlock is held, now that we've acquired the inner lock.
	atomic.AddUint32(m.rheld, 1)

	// Decrement the number waiting to hold the lock.
	atomic.AddUint32(m.rwaiting, ^uint32(0))
}

func (m *RWMutex) RUnlock() {
	// Before we unlock the inner mutex, indicate there is no longer a hold on the rlock.
	atomic.AddUint32(m.rheld, ^uint32(0))
	m.m.RUnlock()
}

var rwmutexCounter uint64

// onFirstUse ensures all values are initialized correctly, sets up a reporting goroutine,
// and registers a finalizer to close the reporting goroutine.
func (m *RWMutex) onFirstUse() {
	id := atomic.AddUint64(&rwmutexCounter, 1)
	RWMutexRegistrationOut <- RWMutexRegistration{ID: id, Stack: debug.Stack()}

	done := make(chan struct{})

	// Ensure values are initialized properly.
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&m.waiting))) == nil {
		atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&m.waiting)), nil, unsafe.Pointer(new(uint32)))
	}
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&m.rwaiting))) == nil {
		atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&m.rwaiting)), nil, unsafe.Pointer(new(uint32)))
	}
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&m.held))) == nil {
		atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&m.held)), nil, unsafe.Pointer(new(uint32)))
	}
	if atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&m.rheld))) == nil {
		atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&m.rheld)), nil, unsafe.Pointer(new(uint32)))
	}

	go reportRWMutex(done, m.waiting, m.rwaiting, m.held, m.rheld, id)

	m.live = new(byte)
	runtime.SetFinalizer(m.live, func(b *byte) {
		close(done)
		RWMutexDeregistrationOut <- id
	})
}

// reportRWMutexInterval determines how frequently an instrumented RWMutex should report its stats.
// For now, it's hardcoded. It may be configurable later.
func reportRWMutexInterval() time.Duration {
	return 25 * time.Millisecond
}

func reportRWMutex(done <-chan struct{}, waiting, rwaiting, held, rheld *uint32, id uint64) {
	t := time.NewTimer(reportRWMutexInterval())
	for {
		select {
		case <-done:
			t.Stop()
			return
		case <-t.C:
			RWMutexStatOut <- RWMutexStat{
				ID:       id,
				Waiting:  atomic.LoadUint32(waiting),
				RWaiting: atomic.LoadUint32(rwaiting),
				Held:     atomic.LoadUint32(held),
				RHeld:    atomic.LoadUint32(rheld),
			}
			t.Reset(reportRWMutexInterval())
		}
	}
}

func (m *RWMutex) RLocker() Locker {
	return (*rlocker)(m)
}

type rlocker RWMutex

func (r *rlocker) Lock()   { (*RWMutex)(r).RLock() }
func (r *rlocker) Unlock() { (*RWMutex)(r).RUnlock() }
