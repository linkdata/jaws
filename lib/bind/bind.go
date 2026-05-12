package bind

import (
	"sync"
)

// New returns a [Binder] with l protecting the value pointed to by p.
//
// If l implements [RWLocker], reads use its read lock. Otherwise reads and
// writes both use l. The pointer p is also exposed as the UI tag.
func New[T comparable](l sync.Locker, p *T) Binder[T] {
	if rl, ok := l.(RWLocker); ok {
		return &binder[T]{RWLocker: rl, ptr: p}
	}
	return &binder[T]{RWLocker: rwlocker{l}, ptr: p}
}
