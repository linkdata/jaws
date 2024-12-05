package jaws

import "time"

type BindingHookGet[T comparable] struct {
	Binder[T]
	BindGetHook[T]
}

func (bind *BindingHookGet[T]) JawsGetLocked(elem *Element) T {
	return bind.BindGetHook(bind.Binder, elem)
}

func (bind *BindingHookGet[T]) JawsGet(elem *Element) T {
	bind.RLock()
	defer bind.RUnlock()
	return bind.JawsGetLocked(elem)
}

func (bind *BindingHookGet[T]) JawsGetString(elem *Element) string {
	return any(bind.JawsGet(elem)).(string)
}

func (bind *BindingHookGet[T]) JawsGetFloat(elem *Element) float64 {
	return any(bind.JawsGet(elem)).(float64)
}

func (bind *BindingHookGet[T]) JawsGetBool(elem *Element) bool {
	return any(bind.JawsGet(elem)).(bool)
}

func (bind *BindingHookGet[T]) JawsGetTime(elem *Element) time.Time {
	return any(bind.JawsGet(elem)).(time.Time)
}

func (bind *BindingHookGet[T]) SetHook(setFn BindSetHook[T]) Binder[T] {
	return &BindingHookSet[T]{
		Binder:      bind,
		BindSetHook: setFn,
	}
}

func (bind *BindingHookGet[T]) GetHook(setFn BindGetHook[T]) Binder[T] {
	return &BindingHookGet[T]{
		Binder:      bind,
		BindGetHook: setFn,
	}
}

func (bind *BindingHookGet[T]) Success(setFn BindSuccessHook) Binder[T] {
	return &BindingHookSet[T]{
		Binder:          bind,
		BindSuccessHook: setFn,
	}
}
