# instrumented-sync

Prototype of a drop-in replacement for Go's standard library `sync` package,
which will report the number of pending locks on a mutex and whether the mutex is currently locked.

**This project is currently pre-alpha status.**

To use, replace

```go
import "sync"
```

with

```go
import "github.com/mark-rushakoff/instrumented-sync"
```

and all of the standard library's `sync` exports will continue to work identically, as the default build behavior is to simply alias to the standard library types.

But if you use `go {build,run,install} -tags=isync`, then `sync.Mutex` will be replaced with an instrumented version that will report on its usage.

To view the output, you must enable a single _drain_.

```go
package mypkg

import "github.com/mark-rushakoff/instrumented-sync/drains/text"

func init() {
  go text.Drain(os.Stdout)
}
```
