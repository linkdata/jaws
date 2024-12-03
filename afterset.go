package jaws

import "time"

type AfterSetFunc[T comparable] func(bind Binding[T]) (err error)

type AfterSetter[T comparable] struct {
	Binding[T]
	AfterSetFunc[T]
}

func (as *AfterSetter[T]) Set(value T) (err error) {
	if err = as.Binding.Set(value); err == nil {
		err = as.AfterSetFunc(as.Binding)
	}
	return
}

func (as *AfterSetter[T]) JawsSet(elem *Element, value T) (err error) {
	return as.Set(value)
}

func (as *AfterSetter[T]) JawsSetString(e *Element, val string) (err error) {
	return as.JawsSet(e, any(val).(T))
}

func (as *AfterSetter[T]) JawsSetFloat(e *Element, val float64) (err error) {
	return as.JawsSet(e, any(val).(T))
}

func (as *AfterSetter[T]) JawsSetBool(e *Element, val bool) (err error) {
	return as.JawsSet(e, any(val).(T))
}

func (as *AfterSetter[T]) JawsSetTime(elem *Element, value time.Time) error {
	return as.JawsSet(elem, any(value).(T))
}

// AfterSet returns a wrapped Binding with a function to call after a
// successful bind.Set.
//
// The functions return value replaces that of Set, and the function may
// use the Binding to inspect and modify the value.
func AfterSet[T comparable](bind Binding[T], fn AfterSetFunc[T]) *AfterSetter[T] {
	return &AfterSetter[T]{
		Binding:      bind,
		AfterSetFunc: fn,
	}
}
