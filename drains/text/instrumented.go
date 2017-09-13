// +build isync

package text

import (
	"fmt"
	"io"

	"github.com/mark-rushakoff/instrumented-sync"
)

type etype uint8

const (
	eRegister etype = iota
	eDeregister
	eStat
)

// Drain drains all instrumented-sync outputs.
// It will block forever.
func Drain(w io.Writer) {
	lines := make(chan string, 512) // Arbitrary buffer size on input lines.
	go drainMutex(lines)
	go drainRWMutex(lines)

	for l := range lines {
		io.WriteString(w, l)
	}
}

func drainMutex(lines chan<- string) {
	// Coalesce all mutex events into a single type,
	// so that we can deduplicate stats and manage our cache.
	type event struct {
		id            uint64
		waiting, held uint32 // only set if stat type
		stack         []byte // only set if registration type
		etype         etype
	}

	events := make(chan event, 256) // Arbitrary buffer size on input events.

	go func() {
		// Coalesce stats to events.
		for s := range sync.MutexStatOut {
			events <- event{
				etype:   eStat,
				id:      s.ID,
				waiting: s.Waiting,
				held:    s.Held,
			}
		}
	}()

	go func() {
		for r := range sync.MutexRegistrationOut {
			events <- event{
				etype: eRegister,
				id:    r.ID,
				stack: r.Stack,
			}
		}
	}()

	go func() {
		for id := range sync.MutexDeregistrationOut {
			events <- event{
				etype: eDeregister,
				id:    id,
			}
		}
	}()

	type stat struct {
		w, h uint32
	}
	cache := map[uint64]stat{}

	for e := range events {
		var s string
		switch e.etype {
		case eRegister:
			s = fmt.Sprintf("New Mutex<%d>: %s\n", e.id, e.stack)
			cache[e.id] = stat{}
		case eDeregister:
			s = fmt.Sprintf("End of Mutex<%d>\n", e.id)
			delete(cache, e.id)
		case eStat:
			stat := stat{w: e.waiting, h: e.held}
			if cache[e.id] == stat {
				continue
			}
			cache[e.id] = stat
			s = fmt.Sprintf("Mutex<%d>: %d waiting, %d held\n", e.id, e.waiting, e.held)
		default:
			panic(fmt.Sprintf("unknown etype: %d", e.etype))
		}
		lines <- s
	}
}

func drainRWMutex(lines chan<- string) {
	// Coalesce all mutex events into a single type,
	// so that we can deduplicate stats and manage our cache.
	type event struct {
		id uint64

		waiting, held   uint32 // only set if stat type
		rwaiting, rheld uint32 // only set if stat type

		stack []byte // only set if registration type

		etype etype
	}

	events := make(chan event, 256) // Arbitrary buffer size on input events.

	go func() {
		// Coalesce stats to events.
		for s := range sync.RWMutexStatOut {
			events <- event{
				etype:    eStat,
				id:       s.ID,
				waiting:  s.Waiting,
				held:     s.Held,
				rwaiting: s.RWaiting,
				rheld:    s.RHeld,
			}
		}
	}()

	go func() {
		for r := range sync.RWMutexRegistrationOut {
			events <- event{
				etype: eRegister,
				id:    r.ID,
				stack: r.Stack,
			}
		}
	}()

	go func() {
		for id := range sync.RWMutexDeregistrationOut {
			events <- event{
				etype: eDeregister,
				id:    id,
			}
		}
	}()

	type stat struct {
		w, h, rw, rh uint32
	}
	cache := map[uint64]stat{}

	for e := range events {
		var s string
		switch e.etype {
		case eRegister:
			s = fmt.Sprintf("New RWMutex<%d>: %s\n", e.id, e.stack)
			cache[e.id] = stat{}
		case eDeregister:
			s = fmt.Sprintf("End of RWMutex<%d>\n", e.id)
			delete(cache, e.id)
		case eStat:
			stat := stat{w: e.waiting, rw: e.rwaiting, h: e.held, rh: e.rheld}
			if cache[e.id] == stat {
				continue
			}
			cache[e.id] = stat
			s = fmt.Sprintf("RWMutex<%d>: %d rwaiting, %d rheld, %d waiting, %d held\n", e.id, e.rwaiting, e.rheld, e.waiting, e.held)
		default:
			panic(fmt.Sprintf("unknown etype: %d", e.etype))
		}
		lines <- s
	}
}
