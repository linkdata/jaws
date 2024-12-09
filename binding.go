package jaws

import "time"

type binding[T comparable] struct {
	lock RWLocker
	ptr  *T
}

func (bind binding[T]) JawsBinderPrev() Binder[T] {
	return nil
}

func (bind binding[T]) JawsGetLocked(*Element) T {
	return *bind.ptr
}

func (bind binding[T]) JawsSetLocked(elem *Element, value T) (err error) {
	if value == *bind.ptr {
		return ErrValueUnchanged
	}
	*bind.ptr = value
	return nil
}

func (bind binding[T]) JawsGet(elem *Element) (value T) {
	bind.lock.RLock()
	value = bind.JawsGetLocked(elem)
	bind.lock.RUnlock()
	return
}

func (bind binding[T]) JawsSet(elem *Element, value T) (err error) {
	bind.lock.Lock()
	err = bind.JawsSetLocked(elem, value)
	bind.lock.Unlock()
	return
}

func (bind binding[T]) JawsGetTag(*Request) any {
	return bind.ptr
}

func (bind binding[T]) Lock() {
	bind.lock.Lock()
}

func (bind binding[T]) Unlock() {
	bind.lock.Unlock()
}

func (bind binding[T]) RLock() {
	bind.lock.RLock()
}

func (bind binding[T]) RUnlock() {
	bind.lock.RUnlock()
}

// SetLocked returns a Binder[T] that will call fn instead of JawsSetLocked.
//
// The lock will be held at this point.
// Do not lock or unlock the Binder within fn. Do not call JawsSet.
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call it's JawsSetLocked first.
func (bind binding[T]) SetLocked(setFn BindSetHook[T]) Binder[T] {
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
func (bind binding[T]) GetLocked(setFn BindGetHook[T]) Binder[T] {
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
func (bind binding[T]) Success(fn any) Binder[T] {
	return &BindingHook[T]{
		Binder:          bind,
		BindSuccessHook: wrapSuccessHook(fn),
	}
}

func (bind binding[T]) JawsGetString(elem *Element) string {
	return any(bind.JawsGet(elem)).(string)
}
func (bind binding[T]) JawsSetString(e *Element, val string) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind binding[T]) JawsGetFloat(elem *Element) float64 {
	return any(bind.JawsGet(elem)).(float64)
}
func (bind binding[T]) JawsSetFloat(e *Element, val float64) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind binding[T]) JawsGetBool(elem *Element) bool {
	return any(bind.JawsGet(elem)).(bool)
}
func (bind binding[T]) JawsSetBool(e *Element, val bool) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind binding[T]) JawsGetTime(elem *Element) time.Time {
	return any(bind.JawsGet(elem)).(time.Time)
}
func (bind binding[T]) JawsSetTime(elem *Element, value time.Time) error {
	return bind.JawsSet(elem, any(value).(T))
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
