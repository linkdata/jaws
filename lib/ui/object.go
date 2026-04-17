package ui

import (
	"errors"
	"html/template"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/tag"
)

// ClickedHook is a function to call when a click event is received.
type ClickedHook func(obj Object, elem *jaws.Element, click jaws.Click) (err error)

// ContextMenuHook is a function to call when a context menu event is received.
type ContextMenuHook func(obj Object, elem *jaws.Element, click jaws.Click) (err error)

// InitialHTMLAttrHook is a function to call when an Element is initially rendered.
type InitialHTMLAttrHook func(obj Object, elem *jaws.Element) (s template.HTMLAttr)

type Object interface {
	bind.HTMLGetter
	tag.TagGetter
	jaws.ClickHandler
	jaws.ContextMenuHandler
	jaws.InitialHTMLAttrHandler

	// Clicked returns an Object that will call fn when JawsClick is invoked.
	Clicked(fn ClickedHook) (newobj Object)

	// ContextMenu returns an Object that will call fn when JawsContextMenu
	// is invoked.
	ContextMenu(fn ContextMenuHook) (newobj Object)

	// InitialHTMLAttr returns an Object that will call fn when
	// JawsInitialHTMLAttr is invoked.
	InitialHTMLAttr(fn InitialHTMLAttrHook) (newobj Object)
}

var _ Object = &object{}

type object struct {
	prev    *object
	handler any
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

func (o *object) InitialHTMLAttr(fn InitialHTMLAttrHook) Object {
	return &object{
		prev:    o,
		handler: fn,
	}
}

func (o *object) JawsGetHTML(e *jaws.Element) (retv template.HTML) {
	for o != nil {
		if h, ok := o.handler.(bind.HTMLGetter); ok {
			retv = h.JawsGetHTML(e)
			break
		}
		o = o.prev
	}
	return
}

func (o *object) JawsClick(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	for o != nil {
		if fn, ok := o.handler.(ClickedHook); ok {
			if err = fn(o, elem, click); !errors.Is(err, jaws.ErrEventUnhandled) {
				break
			}
		}
		o = o.prev
	}
	return
}

func (o *object) JawsContextMenu(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	for o != nil {
		if fn, ok := o.handler.(ContextMenuHook); ok {
			if err = fn(o, elem, click); !errors.Is(err, jaws.ErrEventUnhandled) {
				break
			}
		}
		o = o.prev
	}
	return
}

func (o *object) JawsInitialHTMLAttr(e *jaws.Element) (retv template.HTMLAttr) {
	for o != nil {
		if fn, ok := o.handler.(InitialHTMLAttrHook); ok {
			if s := fn(o, e); s != "" {
				if retv != "" {
					retv += " "
				}
				retv += s
			}
		}
		o = o.prev
	}
	return
}

func (o *object) JawsGetTag(ctx tag.Context) any {
	var tags []any
	for o != nil {
		if h, ok := o.handler.(tag.TagGetter); ok {
			if t := h.JawsGetTag(ctx); t != nil {
				tags = append(tags, t)
			}
		}
		o = o.prev
	}
	switch len(tags) {
	case 0:
		return nil
	case 1:
		return tags[0]
	}
	return tags
}

// New returns a new Object that will render HTML.
// innerHTML is passed to MakeHTMLGetter(), which may or may not provide tags.
func New(innerHTML any) (obj Object) {
	return &object{
		handler: bind.MakeHTMLGetter(innerHTML),
	}
}
