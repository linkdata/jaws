package bind

import (
	"sync"
)

// New returns a [Binder] with l protecting the value pointed to by p.
//
// If l implements [RWLocker], reads use its read lock. Otherwise reads and
// writes both use l. The pointer p is also exposed as the UI tag.
func New[T comparable](l sync.Locker, p *T) Binder[T] {
	return &binder[T]{RWLocker: AsRWLocker(l), ptr: p}
}
