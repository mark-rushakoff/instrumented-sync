// +build isync

package text

import (
	"fmt"
	"io"
	stdsync "sync"

	"github.com/mark-rushakoff/instrumented-sync"
)

// Coalesce all mutex events into a single type,
// so that we can deduplicate stats and manage our cache.
type event struct {
	id            uint64
	waiting, held uint32 // only set if stat type
	stack         []byte // only set if registration type
	etype         etype
}

type etype uint8

const (
	eRegister etype = iota
	eDeregister
	eStat
)

// Drain drains all instrumented-sync outputs.
func Drain(w io.Writer) {
	events := make(chan event, 256) // Arbitrary buffer size on input lines.

	var wg stdsync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

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

	wg.Add(1)
	go func() {
		defer wg.Done()

		for r := range sync.MutexRegistrationOut {
			events <- event{
				etype: eRegister,
				id:    r.ID,
				stack: r.Stack,
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for id := range sync.MutexDeregistrationOut {
			events <- event{
				etype: eDeregister,
				id:    id,
			}
		}
	}()

	go func() {
		wg.Wait()
		close(events)
	}()

	type stat struct {
		w, h uint32
	}
	cache := map[uint64]stat{}

	for e := range events {
		var s string
		switch e.etype {
		case eRegister:
			s = fmt.Sprintf("New mutex<%d>: %s\n", e.id, e.stack)
			cache[e.id] = stat{}
		case eDeregister:
			s = fmt.Sprintf("End of mutex<%d>\n", e.id)
			delete(cache, e.id)
		case eStat:
			stat := stat{w: e.waiting, h: e.held}
			if cache[e.id] == stat {
				continue
			}
			s = fmt.Sprintf("Mutex<%d>: %d waiting, %d held\n", e.id, e.waiting, e.held)
		default:
			panic(fmt.Sprintf("unknown etype: %d", e.etype))
		}
		io.WriteString(w, s)
	}
}
