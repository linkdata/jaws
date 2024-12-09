package jaws

import "time"

type successHooker interface {
	JawsBinderSuccess(elem *Element) (err error)
}

type BindingHook[T comparable] struct {
	Binder[T]
	BindGetHook[T]
	BindSetHook[T]
	BindSuccessHook
}

func (bind *BindingHook[T]) JawsBinderPrev() Binder[T] {
	return bind.Binder
}

func (bind *BindingHook[T]) JawsGetLocked(elem *Element) T {
	if bind.BindGetHook != nil {
		return bind.BindGetHook(bind.Binder, elem)
	}
	return bind.Binder.JawsGetLocked(elem)
}

func (bind *BindingHook[T]) JawsGet(elem *Element) T {
	bind.Binder.RLock()
	defer bind.Binder.RUnlock()
	return bind.JawsGetLocked(elem)
}

func (bind *BindingHook[T]) JawsSetLocked(elem *Element, value T) error {
	if bind.BindSetHook != nil {
		return bind.BindSetHook(bind.Binder, elem, value)
	}
	return bind.Binder.JawsSetLocked(elem, value)
}

func (bind *BindingHook[T]) jawsSetLocking(elem *Element, value T) (err error) {
	bind.Binder.Lock()
	defer bind.Binder.Unlock()
	return bind.JawsSetLocked(elem, value)
}

func callSuccess[T comparable](binder Binder[T], elem *Element) (err error) {
	if prev := binder.JawsBinderPrev(); prev != nil {
		err = callSuccess(prev, elem)
	}
	if err == nil {
		if successer, ok := binder.(successHooker); ok {
			err = successer.JawsBinderSuccess(elem)
		}
	}
	return
}

func (bind *BindingHook[T]) JawsSet(elem *Element, value T) (err error) {
	if err = bind.jawsSetLocking(elem, value); err == nil {
		err = callSuccess(bind, elem)
	}
	return
}

func (bind *BindingHook[T]) JawsGetString(elem *Element) string {
	return any(bind.JawsGet(elem)).(string)
}
func (bind *BindingHook[T]) JawsSetString(e *Element, val string) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind *BindingHook[T]) JawsGetFloat(elem *Element) float64 {
	return any(bind.JawsGet(elem)).(float64)
}
func (bind *BindingHook[T]) JawsSetFloat(e *Element, val float64) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind *BindingHook[T]) JawsGetBool(elem *Element) bool {
	return any(bind.JawsGet(elem)).(bool)
}
func (bind *BindingHook[T]) JawsSetBool(e *Element, val bool) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind *BindingHook[T]) JawsGetTime(elem *Element) time.Time {
	return any(bind.JawsGet(elem)).(time.Time)
}
func (bind *BindingHook[T]) JawsSetTime(elem *Element, value time.Time) error {
	return bind.JawsSet(elem, any(value).(T))
}

// SetLocked returns a Binder[T] that will call fn instead of JawsSetLocked.
//
// The lock will be held at this point.
// Do not lock or unlock the Binder within fn. Do not call JawsSet.
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call it's JawsSetLocked first.
func (bind *BindingHook[T]) SetLocked(setFn BindSetHook[T]) Binder[T] {
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
func (bind *BindingHook[T]) GetLocked(setFn BindGetHook[T]) Binder[T] {
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
func (bind *BindingHook[T]) Success(fn any) Binder[T] {
	return &BindingHook[T]{
		Binder:          bind,
		BindSuccessHook: wrapSuccessHook(fn),
	}
}

func (bind *BindingHook[T]) JawsBinderSuccess(elem *Element) (err error) {
	if bind.BindSuccessHook != nil {
		err = bind.BindSuccessHook(elem)
	}
	return
}
