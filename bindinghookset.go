package jaws

import "time"

type BindingHookSet[T comparable] struct {
	Binder[T]
	BindSetHook[T]
	BindSuccessHook
}

func (bind *BindingHookSet[T]) JawsSetLocked(elem *Element, value T) error {
	if bind.BindSetHook != nil {
		return bind.BindSetHook(bind.Binder, elem, value)
	}
	return bind.Binder.JawsSetLocked(elem, value)
}

func (bind *BindingHookSet[T]) jawsSetLocking(elem *Element, value T) (err error) {
	bind.Lock()
	defer bind.Unlock()
	return bind.JawsSetLocked(elem, value)
}

func (bind *BindingHookSet[T]) JawsSet(elem *Element, value T) (err error) {
	if err = bind.jawsSetLocking(elem, value); err == nil {
		if bind.BindSuccessHook != nil {
			bind.BindSuccessHook()
		}
	}
	return
}

func (bind *BindingHookSet[T]) JawsSetString(e *Element, val string) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind *BindingHookSet[T]) JawsSetFloat(e *Element, val float64) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind *BindingHookSet[T]) JawsSetBool(e *Element, val bool) (err error) {
	return bind.JawsSet(e, any(val).(T))
}

func (bind *BindingHookSet[T]) JawsSetTime(elem *Element, value time.Time) error {
	return bind.JawsSet(elem, any(value).(T))
}

func (bind *BindingHookSet[T]) SetHook(setFn BindSetHook[T]) Binder[T] {
	return &BindingHookSet[T]{
		Binder:      bind,
		BindSetHook: setFn,
	}
}

func (bind *BindingHookSet[T]) GetHook(setFn BindGetHook[T]) Binder[T] {
	return &BindingHookGet[T]{
		Binder:      bind,
		BindGetHook: setFn,
	}
}

func (bind *BindingHookSet[T]) Success(setFn BindSuccessHook) Binder[T] {
	return &BindingHookSet[T]{
		Binder:          bind,
		BindSuccessHook: setFn,
	}
}
