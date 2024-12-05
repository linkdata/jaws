package jaws

import "sync"

type BindSetHook[T comparable] func(bind Binder[T], elem *Element, value T) (err error)
type BindGetHook[T comparable] func(bind Binder[T], elem *Element) (value T)
type BindSuccessHook func()

type Binder[T comparable] interface {
	sync.Locker
	RLocker
	Setter[T]
	BoolSetter
	StringSetter
	FloatSetter
	TimeSetter
	JawsGetLocked(elem *Element) (value T)
	JawsSetLocked(elem *Element, value T) (err error)
	JawsGetTag(*Request) any

	// SetHook returns a Binder[T] that will call fn instead of JawsSetLocked
	// The lock will be held at this point.
	SetHook(fn BindSetHook[T]) (newbind Binder[T])

	// GetHook returns a Binder[T] that will call fn instead of JawsGetLocked
	// The lock (or RLock, if available) will be held at this point.
	GetHook(fn BindGetHook[T]) (newbind Binder[T])

	// Success returns a Binder[T] that will call fn after the value has been set
	// with no errors. No locks are held at this point.
	Success(fn BindSuccessHook) (newbind Binder[T])
}
