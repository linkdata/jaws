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
	next    *object
	handler any
}

func (o *object) addNext(handler any) (first *object) {
	first = o
	for o.next != nil {
		o = o.next
	}
	o.next = &object{handler: handler}
	return first
}

func (o *object) Clicked(fn ClickedHook) Object {
	return o.addNext(fn)
}

func (o *object) ContextMenu(fn ContextMenuHook) Object {
	return o.addNext(fn)
}

func (o *object) JawsGetHTML(e *jaws.Element) template.HTML {
	if h, ok := o.handler.(bind.HTMLGetter); ok {
		return h.JawsGetHTML(e)
	}
	return ""
}

func (o *object) JawsClick(e *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	for o != nil {
		if fn, ok := o.handler.(ClickedHook); ok {
			if err = fn(o, e, click); !errors.Is(err, jaws.ErrEventUnhandled) {
				return
			}
		}
		o = o.next
	}
	return
}

func (o *object) JawsContextMenu(e *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	for o != nil {
		if fn, ok := o.handler.(ContextMenuHook); ok {
			if err = fn(o, e, click); !errors.Is(err, jaws.ErrEventUnhandled) {
				return
			}
		}
		o = o.next
	}
	return
}

func (o *object) JawsGetTag(ctx jtag.Context) any {
	if h, ok := o.handler.(jtag.TagGetter); ok {
		return h.JawsGetTag(ctx)
	}
	return nil
}

// New returns a new Object that will render HTML.
// innerHTML is passed to MakeHTMLGetter(), which may or may not provide tags.
func New(innerHTML any) (obj Object) {
	return &object{
		handler: bind.MakeHTMLGetter(innerHTML),
	}
}
