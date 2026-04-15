package bind

import (
	"errors"
	"fmt"
	"html"
	"html/template"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// SetHook is a function that replaces JawsSetLocked for a Binder.
//
// The lock will be held before calling the function, preferring RLock over Lock, if available.
// Do not lock or unlock the Binder in the function. Do not call JawsSet.
//
// The bind argument is the previous Binder in the chain, and you probably
// want to call it's JawsSetLocked first.
type SetHook[T comparable] func(bind Binder[T], elem *jaws.Element, value T) (err error)

// GetHook is a function that replaces JawsGetLocked for a Binder.
//
// The lock will be held before calling the function, preferring RLock over Lock, if available.
// Do not lock or unlock the Binder in the function. Do not call JawsGet.
//
// The bind argument is the previous Binder in the chain, and you probably
// want to call it's JawsGetLocked first.
type GetHook[T comparable] func(bind Binder[T], elem *jaws.Element) (value T)

// GetHTMLHook is a function to call when JawsGetHTML() is called.
//
// The lock will be held before calling the function, preferring RLock over Lock, if available.
// Do not lock or unlock the Binder in the function. Do not call JawsGet.
type GetHTMLHook[T comparable] func(bind Binder[T], elem *jaws.Element) (s template.HTML)

// ClickedHook is a function to call when a click event is received.
//
// The Binder locks are not held when the function is called.
type ClickedHook[T comparable] func(bind Binder[T], elem *jaws.Element, click jaws.Click) (err error)

// ContextMenuHook is a function to call when a context menu event is received.
//
// The Binder locks are not held when the function is called.
type ContextMenuHook[T comparable] func(bind Binder[T], elem *jaws.Element, click jaws.Click) (err error)

// SuccessHook is a function to call when a call to JawsSet returns with no error.
//
// The Binder locks are not held when the function is called.
//
// Success hooks in a Binder chain are called in the order they were registered.
// If one of them returns an error, that error is returned from JawsSet and
// no more success hooks are called.
type SuccessHook func(*jaws.Element) (err error)

type Binder[T comparable] interface {
	RWLocker
	Setter[T]
	tag.TagGetter
	jaws.ClickHandler
	jaws.ContextMenuHandler

	JawsGetLocked(elem *jaws.Element) (value T)
	JawsSetLocked(elem *jaws.Element, value T) (err error)

	// SetLocked returns a Binder[T] that will call fn instead of JawsSetLocked.
	//
	// The lock will be held at this point.
	// Do not lock or unlock the Binder within fn. Do not call JawsSet.
	//
	// The bind argument to the function is the previous Binder in the chain,
	// and you probably want to call it's JawsSetLocked first.
	SetLocked(fn SetHook[T]) (newbind Binder[T])

	// GetLocked returns a Binder[T] that will call fn instead of JawsGetLocked.
	//
	// The lock will be held at this point, preferring RLock over Lock, if available.
	// Do not lock or unlock the Binder within fn. Do not call JawsGet.
	//
	// The bind argument to the function is the previous Binder in the chain,
	// and you probably want to call it's JawsGetLocked first.
	GetLocked(fn GetHook[T]) (newbind Binder[T])

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

	// GetHTML returns a Binder[T] that will call fn instead of the default
	// escaped fmt.Sprintf("%v", JawsGetLocked(elem)) HTML rendering.
	//
	// The lock will be held at this point, preferring RLock over Lock, if available.
	// Do not lock or unlock the Binder within fn. Do not call JawsGet.
	GetHTML(fn GetHTMLHook[T]) (newbind Binder[T])

	// Clicked returns a Binder[T] that will call fn when JawsClick is invoked.
	//
	// The Binder locks are not held when the function is called.
	Clicked(fn ClickedHook[T]) (newbind Binder[T])

	// ContextMenu returns a Binder[T] that will call fn when JawsContextMenu
	// is invoked.
	//
	// The Binder locks are not held when the function is called.
	ContextMenu(fn ContextMenuHook[T]) (newbind Binder[T])
}

type binder[T comparable] struct {
	prev *binder[T]
	RWLocker
	ptr  *T
	hook any // one of: SetHook[T] GetHook[T] GetHTMLHook[T] ClickedHook[T] ContextMenuHook[T] SuccessHook
}

func (bind *binder[T]) walk(fn func(*binder[T]) bool) bool {
	if bind.prev != nil {
		if bind.prev.walk(fn) {
			return true
		}
	}
	return fn(bind)
}

func (bind *binder[T]) JawsGetLocked(elem *jaws.Element) (value T) {
	if fn, ok := bind.hook.(GetHook[T]); ok {
		value = fn(bind.prev, elem)
	} else if bind.prev != nil {
		value = bind.prev.JawsGetLocked(elem)
	} else {
		value = *bind.ptr
	}
	return
}

func (bind *binder[T]) JawsGet(elem *jaws.Element) (value T) {
	bind.RWLocker.RLock()
	defer bind.RWLocker.RUnlock()
	value = bind.JawsGetLocked(elem)
	return
}

func (bind *binder[T]) jawsGetHTMLLocked(elem *jaws.Element) (s template.HTML) {
	if fn, ok := bind.hook.(GetHTMLHook[T]); ok {
		s = fn(bind.prev, elem)
	} else if bind.prev != nil {
		s = bind.prev.jawsGetHTMLLocked(elem)
	} else {
		s = template.HTML(html.EscapeString(fmt.Sprintf("%v", *bind.ptr))) // #nosec G203
	}
	return
}

func (bind *binder[T]) JawsGetHTML(elem *jaws.Element) (s template.HTML) {
	bind.RWLocker.RLock()
	defer bind.RWLocker.RUnlock()
	s = bind.jawsGetHTMLLocked(elem)
	return
}

func (bind *binder[T]) JawsSetLocked(elem *jaws.Element, value T) (err error) {
	if fn, ok := bind.hook.(SetHook[T]); ok {
		err = fn(bind.prev, elem, value)
	} else if bind.prev != nil {
		err = bind.prev.JawsSetLocked(elem, value)
	} else if value != *bind.ptr {
		*bind.ptr = value
	} else {
		err = jaws.ErrValueUnchanged
	}
	return
}

func (bind *binder[T]) jawsSetLocking(elem *jaws.Element, value T) (err error) {
	bind.RWLocker.Lock()
	defer bind.RWLocker.Unlock()
	err = bind.JawsSetLocked(elem, value)
	return
}

func (bind *binder[T]) callSuccessHooks(elem *jaws.Element) (err error) {
	bind.walk(func(bind *binder[T]) (done bool) {
		var fn SuccessHook
		if fn, done = bind.hook.(SuccessHook); done {
			err = fn(elem)
			done = err != nil
		}
		return
	})
	return
}

func (bind *binder[T]) JawsSet(elem *jaws.Element, value T) (err error) {
	if err = bind.jawsSetLocking(elem, value); err == nil {
		err = bind.callSuccessHooks(elem)
	}
	return
}

func (bind *binder[T]) JawsGetTag(tag.Context) any {
	return bind.ptr
}

func (bind *binder[T]) JawsClick(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	bind.walk(func(bind *binder[T]) (done bool) {
		var fn ClickedHook[T]
		if fn, done = bind.hook.(ClickedHook[T]); done {
			err = fn(bind, elem, click)
			done = !errors.Is(err, jaws.ErrEventUnhandled)
		}
		return
	})
	return
}

func (bind *binder[T]) JawsContextMenu(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	bind.walk(func(bind *binder[T]) (done bool) {
		var fn ContextMenuHook[T]
		if fn, done = bind.hook.(ContextMenuHook[T]); done {
			err = fn(bind, elem, click)
			done = !errors.Is(err, jaws.ErrEventUnhandled)
		}
		return
	})
	return
}

// SetLocked returns a Binder[T] that will call fn instead of JawsSetLocked.
//
// The lock will be held at this point.
// Do not lock or unlock the Binder within fn. Do not call JawsSet.
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call it's JawsSetLocked first.
func (bind *binder[T]) SetLocked(fn SetHook[T]) Binder[T] {
	return &binder[T]{
		prev:     bind,
		RWLocker: bind.RWLocker,
		ptr:      bind.ptr,
		hook:     fn,
	}
}

// GetLocked returns a Binder[T] that will call fn instead of JawsGetLocked.
//
// The lock will be held at this point, preferring RLock over Lock, if available.
// Do not lock or unlock the Binder within fn. Do not call JawsGet.
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call it's JawsGetLocked first.
func (bind *binder[T]) GetLocked(fn GetHook[T]) Binder[T] {
	return &binder[T]{
		prev:     bind,
		RWLocker: bind.RWLocker,
		ptr:      bind.ptr,
		hook:     fn,
	}
}

// GetHTML returns a Binder[T] that will call fn instead of the default escaped
// fmt.Sprintf("%v", JawsGetLocked(elem)) HTML rendering.
//
// The lock will be held at this point, preferring RLock over Lock, if available.
// Do not lock or unlock the Binder within fn. Do not call JawsGet.
func (bind *binder[T]) GetHTML(fn GetHTMLHook[T]) Binder[T] {
	return &binder[T]{
		prev:     bind,
		RWLocker: bind.RWLocker,
		ptr:      bind.ptr,
		hook:     fn,
	}
}

// Clicked returns a Binder[T] that will call fn when JawsClick is invoked.
//
// The Binder locks are not held when the function is called.
func (bind *binder[T]) Clicked(fn ClickedHook[T]) Binder[T] {
	return &binder[T]{
		prev:     bind,
		RWLocker: bind.RWLocker,
		ptr:      bind.ptr,
		hook:     fn,
	}
}

// ContextMenu returns a Binder[T] that will call fn when JawsContextMenu is invoked.
//
// The Binder locks are not held when the function is called.
func (bind *binder[T]) ContextMenu(fn ContextMenuHook[T]) Binder[T] {
	return &binder[T]{
		prev:     bind,
		RWLocker: bind.RWLocker,
		ptr:      bind.ptr,
		hook:     fn,
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
func (bind *binder[T]) Success(fn any) Binder[T] {
	return &binder[T]{
		prev:     bind,
		RWLocker: bind.RWLocker,
		ptr:      bind.ptr,
		hook:     wrapSuccessHook(fn),
	}
}

func wrapSuccessHook(fn any) (hook SuccessHook) {
	switch fn := fn.(type) {
	case func():
		return func(*jaws.Element) error {
			fn()
			return nil
		}
	case func() error:
		return func(*jaws.Element) error {
			return fn()
		}
	case func(*jaws.Element):
		return func(elem *jaws.Element) error {
			fn(elem)
			return nil
		}
	case func(*jaws.Element) error:
		return fn
	}
	panic("Binding[T].Success(): function has wrong signature")
}
