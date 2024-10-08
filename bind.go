package jaws

import (
	"sync"
	"time"
)

// Binding combines a lock with a pointer to a value of type T, and implements Setter[T].
// It also implements BoolSetter, FloatSetter, StringSetter and TimeSetter, but will panic
// if the underlying type T is not correct.
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

func (bind Binding[T]) JawsSetString(e *Element, val string) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind Binding[T]) JawsGetString(e *Element) string {
	return any(bind.JawsGet(e)).(string)
}

func (bind Binding[T]) JawsSetFloat(e *Element, val float64) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind Binding[T]) JawsGetFloat(e *Element) float64 {
	return any(bind.JawsGet(e)).(float64)
}

func (bind Binding[T]) JawsSetBool(e *Element, val bool) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind Binding[T]) JawsGetBool(e *Element) bool {
	return any(bind.JawsGet(e)).(bool)
}

func (bind Binding[T]) JawsGetTime(elem *Element) time.Time {
	return any(bind.JawsGet(elem)).(time.Time)
}

func (bind Binding[T]) JawsSetTime(elem *Element, value time.Time) error {
	return bind.Set(any(value).(T))
}

// Bind returns a Binding[T] with the given sync.Locker (or RWLocker) and a pointer to the underlying value of type T.
// It implements Setter[T]. It also implements BoolSetter, FloatSetter, StringSetter and TimeSetter, but will panic
// if the underlying type T is not correct.
// The pointer will be used as the UI tag.
func Bind[T comparable](l sync.Locker, p *T) Binding[T] {
	return Binding[T]{L: l, P: p}
}
