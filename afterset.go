package jaws

import "time"

type AfterSetter[T comparable] struct {
	Binding[T]
	Func func()
}

func (as *AfterSetter[T]) Set(value T) (err error) {
	if err = as.Binding.Set(value); err == nil {
		as.Func()
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
// successful Set.
func AfterSet[T comparable](bind Binding[T], fn func()) *AfterSetter[T] {
	return &AfterSetter[T]{
		Binding: bind,
		Func:    fn,
	}
}
