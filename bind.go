package jaws

import (
	"sync"
)

// Bind returns a Binder[T] with the given sync.Locker (or RWLocker) and a pointer to the underlying value of type T.
//
// The pointer will be used as the UI tag.
func Bind[T comparable](l sync.Locker, p *T) Binder[T] {
	return Binding[T]{Locker: l, ptr: p}
}
