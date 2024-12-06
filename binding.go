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

func (bind Binding[T]) JawsBinderPrev() Binder[T] {
	return nil
}

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

func (bind Binding[T]) JawsGet(elem *Element) (value T) {
	bind.RLock()
	value = bind.JawsGetLocked(elem)
	bind.RUnlock()
	return
}

func (bind Binding[T]) JawsSet(elem *Element, value T) (err error) {
	bind.Lock()
	err = bind.JawsSetLocked(elem, value)
	bind.Unlock()
	return
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

// SetLocked returns a Binder[T] that will call fn instead of JawsSetLocked.
//
// The lock will be held at this point.
// Do not lock or unlock the Binder within fn. Do not call JawsSet.
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call it's JawsSetLocked first.
func (bind Binding[T]) SetLocked(setFn BindSetHook[T]) Binder[T] {
	return &BindingHook[T]{
		Binder:      bind,
		BindSetHook: setFn,
	}
}

// GetLocked returns a Binder[T] that will call fn instead of JawsGetLocked.
//
// The lock will be held at this point, preferring RLock over Lock, if available.
// Do not lock or unlock the Binder within fn. Do not call JawsGet.
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call it's JawsGetLocked first.
func (bind Binding[T]) GetLocked(setFn BindGetHook[T]) Binder[T] {
	return &BindingHook[T]{
		Binder:      bind,
		BindGetHook: setFn,
	}
}

// Success returns a Binder[T] that will call fn after the value has been set
// with no errors. No locks are held when the function is called.
// If the function returns an error, that will be returned from JawsSet.
//
// The function must have one of the following signatures:
//   - func()
//   - func() error
//   - func(*Element)
//   - func(*Element) error
func (bind Binding[T]) Success(fn any) Binder[T] {
	return &BindingHook[T]{
		Binder:          bind,
		BindSuccessHook: wrapSuccessHook(fn),
	}
}

func wrapSuccessHook(fn any) (hook BindSuccessHook) {
	switch fn := fn.(type) {
	case func():
		return func(*Element) error {
			fn()
			return nil
		}
	case func() error:
		return func(*Element) error {
			return fn()
		}
	case func(*Element):
		return func(elem *Element) error {
			fn(elem)
			return nil
		}
	case func(*Element) error:
		return fn
	}
	panic("Binding[T].Success(): function has wrong signature")
}
