package ui

import (
	"errors"
	"html/template"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/jtag"
)

// ClickedHook is a function to call when a click event is received.
type ClickedHook func(obj Object, elem *jaws.Element, click jaws.Click) (err error)

// ContextMenuHook is a function to call when a context menu event is received.
type ContextMenuHook func(obj Object, elem *jaws.Element, click jaws.Click) (err error)

type Object interface {
	bind.HTMLGetter
	jtag.TagGetter
	jaws.ClickHandler
	jaws.ContextMenuHandler

	// Clicked returns an Object that will call fn when JawsClick is invoked.
	Clicked(fn ClickedHook) (newobj Object)

	// ContextMenu returns an Object that will call fn when JawsContextMenu
	// is invoked.
	ContextMenu(fn ContextMenuHook) (newobj Object)
}

var _ Object = &object{}

type object struct {
	prev    *object
	handler any
}

func (o *object) walk(fn func(o *object) bool) bool {
	if o.prev != nil {
		if o.prev.walk(fn) {
			return true
		}
	}
	return fn(o)
}

func (o *object) Clicked(fn ClickedHook) Object {
	return &object{
		prev:    o,
		handler: fn,
	}
}

func (o *object) ContextMenu(fn ContextMenuHook) Object {
	return &object{
		prev:    o,
		handler: fn,
	}
}

func (o *object) JawsGetHTML(e *jaws.Element) (retv template.HTML) {
	o.walk(func(o *object) (ok bool) {
		var h bind.HTMLGetter
		if h, ok = o.handler.(bind.HTMLGetter); ok {
			retv = h.JawsGetHTML(e)
		}
		return
	})
	return
}

func (o *object) JawsClick(e *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	o.walk(func(o *object) (ok bool) {
		var fn ClickedHook
		if fn, ok = o.handler.(ClickedHook); ok {
			err = fn(o, e, click)
			ok = !errors.Is(err, jaws.ErrEventUnhandled)
		}
		return
	})
	return
}

func (o *object) JawsContextMenu(e *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	o.walk(func(o *object) (ok bool) {
		var fn ContextMenuHook
		if fn, ok = o.handler.(ContextMenuHook); ok {
			err = fn(o, e, click)
			ok = !errors.Is(err, jaws.ErrEventUnhandled)
		}
		return
	})
	return
}

func (o *object) JawsGetTag(ctx jtag.Context) (t any) {
	o.walk(func(o *object) (ok bool) {
		var h jtag.TagGetter
		if h, ok = o.handler.(jtag.TagGetter); ok {
			t = h.JawsGetTag(ctx)
		}
		return
	})
	return
}

// New returns a new Object that will render HTML.
// innerHTML is passed to MakeHTMLGetter(), which may or may not provide tags.
func New(innerHTML any) (obj Object) {
	return &object{
		handler: bind.MakeHTMLGetter(innerHTML),
	}
}
