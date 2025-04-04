package jaws

import (
	"fmt"
	"html/template"
)

type binding[T comparable] struct {
	RWLocker
	ptr *T
}

func (bind binding[T]) JawsBinderPrev() Binder[T] {
	return nil
}

func (bind binding[T]) JawsGetLocked(*Element) T {
	return *bind.ptr
}

func (bind binding[T]) JawsGet(elem *Element) (value T) {
	bind.RWLocker.RLock()
	value = bind.JawsGetLocked(elem)
	bind.RWLocker.RUnlock()
	return
}

func (bind binding[T]) JawsGetAny(elem *Element) (value any) {
	return bind.JawsGet(elem)
}

func (bind binding[T]) JawsSetLocked(elem *Element, value T) (err error) {
	if value == *bind.ptr {
		return ErrValueUnchanged
	}
	*bind.ptr = value
	return nil
}

func (bind binding[T]) JawsSet(elem *Element, value T) (err error) {
	bind.RWLocker.Lock()
	err = bind.JawsSetLocked(elem, value)
	bind.RWLocker.Unlock()
	return
}

func (bind binding[T]) JawsSetAny(elem *Element, value any) (err error) {
	return bind.JawsSet(elem, value.(T))
}

func (bind binding[T]) JawsGetTag(*Request) any {
	return bind.ptr
}

// SetLocked returns a Binder[T] that will call fn instead of JawsSetLocked.
//
// The lock will be held at this point.
// Do not lock or unlock the Binder within fn. Do not call JawsSet.
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call it's JawsSetLocked first.
func (bind binding[T]) SetLocked(fn BindSetHook[T]) Binder[T] {
	return bindingHook[T]{
		Binder: bind,
		hook:   fn,
	}
}

// GetLocked returns a Binder[T] that will call fn instead of JawsGetLocked.
//
// The lock will be held at this point, preferring RLock over Lock, if available.
// Do not lock or unlock the Binder within fn. Do not call JawsGet.
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call it's JawsGetLocked first.
func (bind binding[T]) GetLocked(fn BindGetHook[T]) Binder[T] {
	return bindingHook[T]{
		Binder: bind,
		hook:   fn,
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
	return bindingHook[T]{
		Binder: bind,
		hook:   wrapSuccessHook(fn),
	}
}

// Format returns a Getter[string] using fmt.Sprintf(f, JawsGet[T](elem))
func (bind binding[T]) Format(f string) (getter Getter[string]) {
	return StringGetterFunc(func(elem *Element) (s string) { return fmt.Sprintf(f, bind.JawsGet(elem)) }, bind)
}

// FormatHTML returns a HTMLGetter using fmt.Sprintf(f, JawsGet[T](elem)).
// Ensure that the generated string is valid HTML.
func (bind binding[T]) FormatHTML(f string) (getter HTMLGetter) {
	return HTMLGetterFunc(func(elem *Element) (tmpl template.HTML) {
		return template.HTML( /*#nosec G203*/ fmt.Sprintf(f, bind.JawsGet(elem)))
	}, bind)
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
