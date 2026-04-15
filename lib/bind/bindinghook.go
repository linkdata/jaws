package bind

import (
	"errors"
	"fmt"
	"html/template"

	"github.com/linkdata/jaws"
)

type bindingHook[T comparable] struct {
	Binder[T]
	hook any // one of: BindGetHook[T] BindSetHook[T] BindClickedHook[T] BindContextMenuHook[T] BindSuccessHook
}

func (bind bindingHook[T]) JawsBinderPrev() Binder[T] {
	return bind.Binder
}

func (bind bindingHook[T]) JawsGetLocked(elem *jaws.Element) T {
	if fn, ok := bind.hook.(BindGetHook[T]); ok {
		return fn(bind.Binder, elem)
	}
	return bind.Binder.JawsGetLocked(elem)
}

func (bind bindingHook[T]) JawsGet(elem *jaws.Element) T {
	bind.RLock()
	defer bind.RUnlock()
	return bind.JawsGetLocked(elem)
}

func (bind bindingHook[T]) JawsSetLocked(elem *jaws.Element, value T) error {
	if fn, ok := bind.hook.(BindSetHook[T]); ok {
		return fn(bind.Binder, elem, value)
	}
	return bind.Binder.JawsSetLocked(elem, value)
}

func (bind bindingHook[T]) jawsSetLocking(elem *jaws.Element, value T) (err error) {
	bind.Lock()
	defer bind.Unlock()
	return bind.JawsSetLocked(elem, value)
}

type callChainType int

const (
	callChainInvalid = callChainType(iota)
	callChainSuccess
	callChainClicked
	callChainContextMenu
)

func callChain[T comparable](binder Binder[T], elem *jaws.Element, kind callChainType, param any) (err error) {
	if prev := binder.JawsBinderPrev(); prev != nil {
		err = callChain(prev, elem, kind, param)
	} else if kind == callChainClicked || kind == callChainContextMenu {
		err = jaws.ErrEventUnhandled
	}
	if bh, ok := binder.(bindingHook[T]); ok {
		switch kind {
		case callChainSuccess:
			if err == nil {
				if fn, ok := bh.hook.(BindSuccessHook); ok {
					err = fn(elem)
				}
			}
		case callChainClicked:
			if errors.Is(err, jaws.ErrEventUnhandled) {
				if fn, ok := bh.hook.(BindClickedHook[T]); ok {
					err = fn(bh, elem, param.(jaws.Click))
				}
			}
		case callChainContextMenu:
			if errors.Is(err, jaws.ErrEventUnhandled) {
				if fn, ok := bh.hook.(BindContextMenuHook[T]); ok {
					err = fn(bh, elem, param.(jaws.Click))
				}
			}
		}
	}
	return
}

func (bind bindingHook[T]) JawsSet(elem *jaws.Element, value T) (err error) {
	if err = bind.jawsSetLocking(elem, value); err == nil {
		err = callChain(bind, elem, callChainSuccess, nil)
	}
	return
}

func (bind bindingHook[T]) JawsClick(elem *jaws.Element, click jaws.Click) (err error) {
	err = callChain(bind, elem, callChainClicked, click)
	return
}

func (bind bindingHook[T]) JawsContextMenu(elem *jaws.Element, click jaws.Click) (err error) {
	err = callChain(bind, elem, callChainContextMenu, click)
	return
}

// SetLocked returns a Binder[T] that will call fn instead of JawsSetLocked.
//
// The lock will be held at this point.
// Do not lock or unlock the Binder within fn. Do not call JawsSet.
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call it's JawsSetLocked first.
func (bind bindingHook[T]) SetLocked(setFn BindSetHook[T]) Binder[T] {
	return bindingHook[T]{
		Binder: bind,
		hook:   setFn,
	}
}

// GetLocked returns a Binder[T] that will call fn instead of JawsGetLocked.
//
// The lock will be held at this point, preferring RLock over Lock, if available.
// Do not lock or unlock the Binder within fn. Do not call JawsGet.
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call it's JawsGetLocked first.
func (bind bindingHook[T]) GetLocked(setFn BindGetHook[T]) Binder[T] {
	return bindingHook[T]{
		Binder: bind,
		hook:   setFn,
	}
}

// Clicked returns a Binder[T] that will call fn when JawsClick is invoked.
//
// The Binder locks are not held when the function is called.
func (bind bindingHook[T]) Clicked(fn BindClickedHook[T]) Binder[T] {
	return bindingHook[T]{
		Binder: bind,
		hook:   fn,
	}
}

// ContextMenu returns a Binder[T] that will call fn when JawsContextMenu is invoked.
//
// The Binder locks are not held when the function is called.
func (bind bindingHook[T]) ContextMenu(fn BindContextMenuHook[T]) Binder[T] {
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
func (bind bindingHook[T]) Success(fn any) Binder[T] {
	return bindingHook[T]{
		Binder: bind,
		hook:   wrapSuccessHook(fn),
	}
}

// Format returns a Getter[string] using fmt.Sprintf(f, JawsGet[T](elem))
func (bind bindingHook[T]) Format(f string) (getter Getter[string]) {
	return StringGetterFunc(func(elem *jaws.Element) (s string) { return fmt.Sprintf(f, bind.JawsGet(elem)) }, bind)
}

// FormatHTML returns a HTMLGetter using fmt.Sprintf(f, JawsGet[T](elem)).
// Ensure that the generated string is valid HTML.
func (bind bindingHook[T]) FormatHTML(f string) (getter HTMLGetter) {
	return HTMLGetterFunc(func(elem *jaws.Element) (tmpl template.HTML) {
		return template.HTML( /*#nosec G203*/ fmt.Sprintf(f, bind.JawsGet(elem)))
	}, bind)
}
