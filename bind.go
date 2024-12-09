package jaws

import (
	"sync"
)

// Bind returns a Binder[T] with the given sync.Locker (or RWLocker) and a pointer to the underlying value of type T.
//
// The pointer will be used as the UI tag.
func Bind[T comparable](l sync.Locker, p *T) Binder[T] {
	if rl, ok := l.(RWLocker); ok {
		return binding[T]{lock: rl, ptr: p}
	}
	return binding[T]{lock: rwlocker{l}, ptr: p}
}
