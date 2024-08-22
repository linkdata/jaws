package jaws

import "sync"

// Binding combines a lock with a pointer to a value, and implements the Setter interface.
type Binding[T comparable] struct {
	L sync.Locker
	P *T
	_ T
}

func (bind Binding[T]) JawsGet(elem *Element) (value T) {
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

func (bind Binding[T]) JawsSet(elem *Element, value T) (err error) {
	bind.L.Lock()
	if value != *bind.P {
		*bind.P = value
	} else {
		err = ErrValueUnchanged
	}
	bind.L.Unlock()
	return
}

func Bind[T comparable](l sync.Locker, p *T) Binding[T] {
	return Binding[T]{L: l, P: p}
}
