package bind

import (
	"sync"
)

// New returns a Binder[T] with the given sync.Locker (or RWLocker) and a pointer to the underlying value of type T.
//
// The pointer will be used as the UI tag.
func New[T comparable](l sync.Locker, p *T) Binder[T] {
	if rl, ok := l.(RWLocker); ok {
		return &binder[T]{RWLocker: rl, ptr: p}
	}
	return &binder[T]{RWLocker: rwlocker{l}, ptr: p}
}
