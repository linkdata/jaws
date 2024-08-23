package jaws

import (
	"sync"
)

// Binding combines a lock with a pointer to a value, and implements the Setter interface.
type Binding[T comparable] struct {
	L sync.Locker
	P *T
}

func (bind Binding[T]) Get() (value T) {
	if rl, ok := bind.L.(RLocker); ok {
		rl.RLock()
		value = *bind.P
		rl.RUnlock()
	} else {
		bind.L.Lock()
		value = *bind.P
		bind.L.Unlock()
	}
	return
}

func (bind Binding[T]) Set(value T) (err error) {
	bind.L.Lock()
	if value != *bind.P {
		*bind.P = value
	} else {
		err = ErrValueUnchanged
	}
	bind.L.Unlock()
	return
}

func (bind Binding[T]) JawsGet(elem *Element) T {
	return bind.Get()
}

func (bind Binding[T]) JawsSet(elem *Element, value T) error {
	return bind.Set(value)
}

func (bind Binding[T]) JawsGetTag(*Request) any {
	return bind.P
}

// Bind returns a Binding with the given sync.Locker (or RWLocker) and a pointer to the underlying value.
// It implements Setter. The pointer will be used as the UI tag.
func Bind[T comparable](l sync.Locker, p *T) Binding[T] {
	return Binding[T]{L: l, P: p}
}
