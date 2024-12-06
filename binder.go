package jaws

import "sync"

// BindSetHook is a function that replaces JawsSetLocked for a Binder.
//
// The lock will be held before calling the function, preferring RLock over Lock, if available.
// Do not lock or unlock the Binder in the function. Do not call JawsSet.
//
// The bind argument is the previous Binder in the chain, and you probably
// want to call it's JawsSetLocked first.
type BindSetHook[T comparable] func(bind Binder[T], elem *Element, value T) (err error)

// BindGetHook is a function that replaces JawsGetLocked for a Binder.
//
// The lock will be held before calling the function, preferring RLock over Lock, if available.
// Do not lock or unlock the Binder in the function. Do not call JawsGet.
//
// The bind argument is the previous Binder in the chain, and you probably
// want to call it's JawsGetLocked first.
type BindGetHook[T comparable] func(bind Binder[T], elem *Element) (value T)

// BindSuccessHook is a function to call when a call to JawsSet returns with no error.
//
// The Binder locks are not held when the function is called.
//
// Success hooks in a Binder chain are called in the order they were registered.
// If one of them returns an error, that error is returned from JawsSet and
// no more success hooks are called.
type BindSuccessHook func(*Element) (err error)

type Binder[T comparable] interface {
	sync.Locker
	RLocker
	Setter[T]
	JawsGetTag(*Request) any

	JawsBinderPrev() Binder[T] // returns the previous Binder in the chain, or nil
	JawsGetLocked(elem *Element) (value T)
	JawsSetLocked(elem *Element, value T) (err error)

	// SetLocked returns a Binder[T] that will call fn instead of JawsSetLocked.
	//
	// The lock will be held at this point.
	// Do not lock or unlock the Binder within fn. Do not call JawsSet.
	//
	// The bind argument to the function is the previous Binder in the chain,
	// and you probably want to call it's JawsSetLocked first.
	SetLocked(fn BindSetHook[T]) (newbind Binder[T])

	// GetLocked returns a Binder[T] that will call fn instead of JawsGetLocked.
	//
	// The lock will be held at this point, preferring RLock over Lock, if available.
	// Do not lock or unlock the Binder within fn. Do not call JawsGet.
	//
	// The bind argument to the function is the previous Binder in the chain,
	// and you probably want to call it's JawsGetLocked first.
	GetLocked(fn BindGetHook[T]) (newbind Binder[T])

	// Success returns a Binder[T] that will call fn after the value has been set
	// with no errors. No locks are held when the function is called.
	// If the function returns an error, that will be returned from JawsSet.
	//
	// The function must have one of the following signatures:
	//  * func()
	//  * func() error
	//  * func(*Element)
	//  * func(*Element) error
	Success(fn any) (newbind Binder[T])
}
