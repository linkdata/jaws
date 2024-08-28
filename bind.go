package jaws

import (
	"sync"
	"time"
)

// Binding combines a lock with a pointer to a value of type T, and implements
// the Setter[T], BoolSetter, FloatSetter, StringSetter and TimeSetter interfaces.
type Binding[T comparable] struct {
	L sync.Locker
	P *T
}

var (
	_ BoolSetter   = Binding[bool]{}
	_ FloatSetter  = Binding[float64]{}
	_ StringSetter = Binding[string]{}
	_ TimeSetter   = Binding[time.Time]{}
)

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

func (bind Binding[string]) JawsSetString(e *Element, val string) (err error) {
	return bind.JawsSet(e, val)
}

func (bind Binding[string]) JawsGetString(e *Element) string {
	return bind.JawsGet(e)
}

func (bind Binding[float64]) JawsSetFloat(e *Element, val float64) (err error) {
	return bind.JawsSet(e, val)
}

func (bind Binding[float64]) JawsGetFloat(e *Element) float64 {
	return bind.JawsGet(e)
}

func (bind Binding[bool]) JawsSetBool(e *Element, val bool) (err error) {
	return bind.JawsSet(e, val)
}

func (bind Binding[bool]) JawsGetBool(e *Element) bool {
	return bind.JawsGet(e)
}

type Time = time.Time

func (bind Binding[Time]) JawsGetTime(elem *Element) time.Time {
	return any(bind.JawsGet(elem)).(time.Time)
}

func (bind Binding[Time]) JawsSetTime(elem *Element, value time.Time) error {
	return bind.Set(any(value).(Time))
}

// Bind returns a Binding[T] with the given sync.Locker (or RWLocker) and a pointer to the underlying value of type T.
// It implements Setter[T], BoolSetter, FloatSetter, StringSetter and TimeSetter.
// The pointer will be used as the UI tag.
func Bind[T comparable](l sync.Locker, p *T) Binding[T] {
	return Binding[T]{L: l, P: p}
}
