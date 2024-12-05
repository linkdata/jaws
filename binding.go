package jaws

import (
	"sync"
	"time"
)

// Binding combines a lock with a pointer to a value of type T, and implements Setter[T].
// It also implements BoolSetter, FloatSetter, StringSetter and TimeSetter, but will panic
// if the underlying type T is not correct.
type Binding[T comparable] struct {
	sync.Locker
	ptr *T
}

var (
	_ BoolSetter   = Binding[bool]{}
	_ FloatSetter  = Binding[float64]{}
	_ StringSetter = Binding[string]{}
	_ TimeSetter   = Binding[time.Time]{}
)

func (bind Binding[T]) JawsGetLocked(*Element) T {
	return *bind.ptr
}

func (bind Binding[T]) JawsSetLocked(elem *Element, value T) (err error) {
	if value == *bind.ptr {
		return ErrValueUnchanged
	}
	*bind.ptr = value
	return nil
}

func (bind Binding[T]) JawsGet(elem *Element) T {
	bind.RLock()
	defer bind.RUnlock()
	return bind.JawsGetLocked(elem)
}

func (bind Binding[T]) JawsSet(elem *Element, value T) error {
	bind.Lock()
	defer bind.Unlock()
	return bind.JawsSetLocked(elem, value)
}

func (bind Binding[T]) JawsGetTag(*Request) any {
	return bind.ptr
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
	return bind.JawsSet(elem, any(value).(T))
}

func (bind Binding[T]) RLock() {
	if rl, ok := bind.Locker.(RLocker); ok {
		rl.RLock()
	} else {
		bind.Lock()
	}
}

func (bind Binding[T]) RUnlock() {
	if rl, ok := bind.Locker.(RLocker); ok {
		rl.RUnlock()
	} else {
		bind.Unlock()
	}
}

func (bind Binding[T]) SetHook(setFn BindSetHook[T]) Binder[T] {
	return &BindingHookSet[T]{
		Binder:      bind,
		BindSetHook: setFn,
	}
}

func (bind Binding[T]) GetHook(setFn BindGetHook[T]) Binder[T] {
	return &BindingHookGet[T]{
		Binder:      bind,
		BindGetHook: setFn,
	}
}

func (bind Binding[T]) Success(setFn BindSuccessHook) Binder[T] {
	return &BindingHookSet[T]{
		Binder:          bind,
		BindSuccessHook: setFn,
	}
}
