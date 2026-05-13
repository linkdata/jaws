package bind

import (
	"errors"
	"fmt"
	"html"
	"html/template"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// SetHook is a function that replaces [Binder.JawsSetLocked].
//
// The lock will be held before calling the function, preferring RLock over Lock, if available.
// Do not lock or unlock the [Binder] in the function. Do not call [Binder.JawsSet].
//
// The bind argument is the previous Binder in the chain, and you probably
// want to call its [Binder.JawsSetLocked] first.
type SetHook[T comparable] func(bind Binder[T], elem *jaws.Element, value T) (err error)

// GetHook is a function that replaces [Binder.JawsGetLocked].
//
// The lock will be held before calling the function, preferring RLock over Lock, if available.
// Do not lock or unlock the [Binder] in the function. Do not call [Getter.JawsGet].
//
// The bind argument is the previous Binder in the chain, and you probably
// want to call its [Binder.JawsGetLocked] first.
type GetHook[T comparable] func(bind Binder[T], elem *jaws.Element) (value T)

// GetHTMLHook is a function to call when [HTMLGetter.JawsGetHTML] is called.
//
// The lock will be held before calling the function, preferring RLock over Lock, if available.
// Do not lock or unlock the [Binder] in the function. Do not call [Getter.JawsGet].
type GetHTMLHook[T comparable] func(bind Binder[T], elem *jaws.Element) (s template.HTML)

// ClickedHook is a function to call when a click event is received.
//
// The [Binder] locks are not held when the function is called.
type ClickedHook[T comparable] func(bind Binder[T], elem *jaws.Element, click jaws.Click) (err error)

// ContextMenuHook is a function to call when a context menu event is received.
//
// The [Binder] locks are not held when the function is called.
type ContextMenuHook[T comparable] func(bind Binder[T], elem *jaws.Element, click jaws.Click) (err error)

// InitialHTMLAttrHook is a function to call when an Element is initially rendered.
//
// The lock will be held at this point, preferring RLock over Lock, if available.
// Do not lock or unlock the [Binder] within fn. Do not call [Getter.JawsGet].
type InitialHTMLAttrHook[T comparable] func(bind Binder[T], elem *jaws.Element) (s template.HTMLAttr)

// SuccessHook is a function to call when [Setter.JawsSet] returns with no error.
//
// The [Binder] locks are not held when the function is called.
//
// Success hooks in a [Binder] chain are called in reverse registration order.
// If one of them returns an error, that error is returned from [Setter.JawsSet] and
// no more success hooks are called.
type SuccessHook func(elem *jaws.Element) (err error)

// Formatter customizes [Binder.Format] output for a value.
type Formatter interface {
	Format(string) string
}

// Binder binds a comparable Go value to JaWS getter, setter, tag and event
// interfaces.
//
// Binder methods are safe for concurrent use when the locker passed to [New]
// is safe for concurrent use.
type Binder[T comparable] interface {
	RWLocker
	Setter[T]
	tag.TagGetter
	jaws.ClickHandler
	jaws.ContextMenuHandler
	jaws.InitialHTMLAttrHandler

	JawsGetLocked(elem *jaws.Element) (value T)
	JawsSetLocked(elem *jaws.Element, value T) (err error)

	// JawsInitialHTMLAttrLocked returns the initial HTML attribute while
	// the Binder lock is held.
	JawsInitialHTMLAttrLocked(elem *jaws.Element) (s template.HTMLAttr)

	// SetLocked returns a [Binder] that will call fn instead of [Binder.JawsSetLocked].
	//
	// The lock will be held at this point.
	// Do not lock or unlock the [Binder] within fn. Do not call [Setter.JawsSet].
	//
	// The bind argument to the function is the previous Binder in the chain,
	// and you probably want to call its [Binder.JawsSetLocked] first.
	SetLocked(fn SetHook[T]) (newbind Binder[T])

	// GetLocked returns a [Binder] that will call fn instead of [Binder.JawsGetLocked].
	//
	// The lock will be held at this point, preferring RLock over Lock, if available.
	// Do not lock or unlock the [Binder] within fn. Do not call [Getter.JawsGet].
	//
	// The bind argument to the function is the previous Binder in the chain,
	// and you probably want to call its [Binder.JawsGetLocked] first.
	GetLocked(fn GetHook[T]) (newbind Binder[T])

	// Success returns a [Binder] that will call fn after the value has been set
	// with no errors. No locks are held when the function is called.
	// If the function returns an error, that will be returned from [Setter.JawsSet].
	//
	// The function must have one of the following signatures:
	//  * func()
	//  * func() error
	//  * func(*Element)
	//  * func(*Element) error
	Success(fn any) (newbind Binder[T])

	// GetHTML returns a [Binder] that will call fn instead of the default
	// escaped fmt.Sprintf("%v", JawsGetLocked(elem)) HTML rendering.
	//
	// The lock will be held at this point, preferring RLock over Lock, if available.
	// Do not lock or unlock the [Binder] within fn. Do not call [Getter.JawsGet].
	GetHTML(fn GetHTMLHook[T]) (newbind Binder[T])

	// Format returns a [Binder] that implements [HTMLGetter] and
	// calls html.EscapeString on either fmt.Sprintf(format, JawsGetLocked(elem))
	// or, if T implements [Formatter], T.Format(format).
	Format(format string) (newbind Binder[T])

	// Clicked returns a [Binder] that will call fn when [jaws.ClickHandler.JawsClick] is invoked.
	//
	// The [Binder] locks are not held when the function is called.
	Clicked(fn ClickedHook[T]) (newbind Binder[T])

	// ContextMenu returns a [Binder] that will call fn when
	// [jaws.ContextMenuHandler.JawsContextMenu] is invoked.
	//
	// The [Binder] locks are not held when the function is called.
	ContextMenu(fn ContextMenuHook[T]) (newbind Binder[T])

	// InitialHTMLAttr returns a [Binder] that will call fn when
	// [jaws.InitialHTMLAttrHandler.JawsInitialHTMLAttr] is invoked.
	//
	// The lock will be held at this point, preferring RLock over Lock, if available.
	// Do not lock or unlock the [Binder] within fn. Do not call [Getter.JawsGet].
	// To call the previous handler in the chain, call [Binder.JawsInitialHTMLAttrLocked].
	InitialHTMLAttr(fn InitialHTMLAttrHook[T]) (newbind Binder[T])
}

type binder[T comparable] struct {
	prev *binder[T]
	RWLocker
	ptr  *T
	hook any
}

func (b *binder[T]) JawsGetLocked(elem *jaws.Element) (value T) {
	if fn, ok := b.hook.(GetHook[T]); ok {
		value = fn(b.prev, elem)
	} else if b.prev != nil {
		value = b.prev.JawsGetLocked(elem)
	} else {
		value = *b.ptr
	}
	return
}

func (b *binder[T]) JawsGet(elem *jaws.Element) (value T) {
	b.RWLocker.RLock()
	defer b.RWLocker.RUnlock()
	value = b.JawsGetLocked(elem)
	return
}

func (b *binder[T]) jawsGetHTMLLocked(elem *jaws.Element) template.HTML {
	for bnd := b; bnd != nil; bnd = bnd.prev {
		switch hook := (bnd.hook).(type) {
		case GetHTMLHook[T]:
			return hook(bnd, elem)
		case string:
			var s string
			v := b.JawsGetLocked(elem)
			if fm, ok := any(v).(Formatter); ok {
				s = fm.Format(hook)
			} else {
				s = fmt.Sprintf(hook, v)
			}
			return template.HTML(html.EscapeString(s)) // #nosec G203
		}
	}
	return template.HTML(html.EscapeString(fmt.Sprint(b.JawsGetLocked(elem)))) // #nosec G203
}

func (b *binder[T]) JawsGetHTML(elem *jaws.Element) (s template.HTML) {
	b.RWLocker.RLock()
	defer b.RWLocker.RUnlock()
	s = b.jawsGetHTMLLocked(elem)
	return
}

func (b *binder[T]) JawsInitialHTMLAttrLocked(elem *jaws.Element) (s template.HTMLAttr) {
	if fn, ok := b.hook.(InitialHTMLAttrHook[T]); ok {
		s = fn(b.prev, elem)
	} else if b.prev != nil {
		s = b.prev.JawsInitialHTMLAttrLocked(elem)
	}
	return
}

func (b *binder[T]) JawsInitialHTMLAttr(elem *jaws.Element) (s template.HTMLAttr) {
	b.RWLocker.RLock()
	defer b.RWLocker.RUnlock()
	s = b.JawsInitialHTMLAttrLocked(elem)
	return
}

func (b *binder[T]) JawsSetLocked(elem *jaws.Element, value T) (err error) {
	if fn, ok := b.hook.(SetHook[T]); ok {
		err = fn(b.prev, elem, value)
	} else if b.prev != nil {
		err = b.prev.JawsSetLocked(elem, value)
	} else if value != *b.ptr {
		*b.ptr = value
	} else {
		err = jaws.ErrValueUnchanged
	}
	return
}

func (b *binder[T]) jawsSetLocking(elem *jaws.Element, value T) (err error) {
	b.RWLocker.Lock()
	defer b.RWLocker.Unlock()
	err = b.JawsSetLocked(elem, value)
	return
}

func (b *binder[T]) callSuccessHooks(elem *jaws.Element) (err error) {
	for b != nil {
		if fn, ok := b.hook.(SuccessHook); ok {
			if err = fn(elem); err != nil {
				break
			}
		}
		b = b.prev
	}
	return
}

func (b *binder[T]) JawsSet(elem *jaws.Element, value T) (err error) {
	if err = b.jawsSetLocking(elem, value); err == nil {
		err = b.callSuccessHooks(elem)
	}
	return
}

func (b *binder[T]) JawsGetTag(tag.Context) any {
	return b.ptr
}

func (b *binder[T]) JawsClick(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	for b != nil {
		if fn, ok := b.hook.(ClickedHook[T]); ok {
			err = fn(b, elem, click)
			if !errors.Is(err, jaws.ErrEventUnhandled) {
				break
			}
		}
		b = b.prev
	}
	return
}

func (b *binder[T]) JawsContextMenu(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	for b != nil {
		if fn, ok := b.hook.(ContextMenuHook[T]); ok {
			err = fn(b, elem, click)
			if !errors.Is(err, jaws.ErrEventUnhandled) {
				break
			}
		}
		b = b.prev
	}
	return
}

// SetLocked returns a [Binder] that will call fn instead of [Binder.JawsSetLocked].
//
// The lock will be held at this point.
// Do not lock or unlock the [Binder] within fn. Do not call [Setter.JawsSet].
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call its [Binder.JawsSetLocked] first.
func (b *binder[T]) SetLocked(fn SetHook[T]) Binder[T] {
	return &binder[T]{
		prev:     b,
		RWLocker: b.RWLocker,
		ptr:      b.ptr,
		hook:     fn,
	}
}

// GetLocked returns a [Binder] that will call fn instead of [Binder.JawsGetLocked].
//
// The lock will be held at this point, preferring RLock over Lock, if available.
// Do not lock or unlock the [Binder] within fn. Do not call [Getter.JawsGet].
//
// The bind argument to the function is the previous Binder in the chain,
// and you probably want to call its [Binder.JawsGetLocked] first.
func (b *binder[T]) GetLocked(fn GetHook[T]) Binder[T] {
	return &binder[T]{
		prev:     b,
		RWLocker: b.RWLocker,
		ptr:      b.ptr,
		hook:     fn,
	}
}

// Format returns a [Binder] that implements [HTMLGetter] and
// calls html.EscapeString on either fmt.Sprintf(format, JawsGetLocked(elem))
// or, if T implements [Formatter], T.Format(format).
func (b *binder[T]) Format(format string) Binder[T] {
	return &binder[T]{
		prev:     b,
		RWLocker: b.RWLocker,
		ptr:      b.ptr,
		hook:     format,
	}
}

// GetHTML returns a [Binder] that will call fn instead of the default escaped
// fmt.Sprintf("%v", JawsGetLocked(elem)) HTML rendering.
//
// The lock will be held at this point, preferring RLock over Lock, if available.
// Do not lock or unlock the [Binder] within fn. Do not call [Getter.JawsGet].
func (b *binder[T]) GetHTML(fn GetHTMLHook[T]) Binder[T] {
	return &binder[T]{
		prev:     b,
		RWLocker: b.RWLocker,
		ptr:      b.ptr,
		hook:     fn,
	}
}

// Clicked returns a [Binder] that will call fn when [jaws.ClickHandler.JawsClick] is invoked.
//
// The [Binder] locks are not held when the function is called.
func (b *binder[T]) Clicked(fn ClickedHook[T]) Binder[T] {
	return &binder[T]{
		prev:     b,
		RWLocker: b.RWLocker,
		ptr:      b.ptr,
		hook:     fn,
	}
}

// ContextMenu returns a [Binder] that will call fn when
// [jaws.ContextMenuHandler.JawsContextMenu] is invoked.
//
// The [Binder] locks are not held when the function is called.
func (b *binder[T]) ContextMenu(fn ContextMenuHook[T]) Binder[T] {
	return &binder[T]{
		prev:     b,
		RWLocker: b.RWLocker,
		ptr:      b.ptr,
		hook:     fn,
	}
}

// InitialHTMLAttr returns a [Binder] that will call fn when
// [jaws.InitialHTMLAttrHandler.JawsInitialHTMLAttr] is invoked.
//
// The lock will be held at this point, preferring RLock over Lock, if available.
// Do not lock or unlock the [Binder] within fn. Do not call [Getter.JawsGet].
func (b *binder[T]) InitialHTMLAttr(fn InitialHTMLAttrHook[T]) Binder[T] {
	return &binder[T]{
		prev:     b,
		RWLocker: b.RWLocker,
		ptr:      b.ptr,
		hook:     fn,
	}
}

// Success returns a [Binder] that will call fn after the value has been set
// with no errors. No locks are held when the function is called.
// If the function returns an error, that will be returned from [Setter.JawsSet].
//
// The function must have one of the following signatures:
//   - func()
//   - func() error
//   - func(*Element)
//   - func(*Element) error
func (b *binder[T]) Success(fn any) Binder[T] {
	return &binder[T]{
		prev:     b,
		RWLocker: b.RWLocker,
		ptr:      b.ptr,
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
	panic("Binder[T].Success(): function has wrong signature")
}
