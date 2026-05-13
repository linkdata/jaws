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

// InitialHTMLAttrHook is a function to call when an [jaws.Element] is initially rendered.
type InitialHTMLAttrHook func(obj Object, elem *jaws.Element) (s template.HTMLAttr)

// Object is a chainable UI object that combines HTML rendering, tags and
// optional event handlers.
type Object interface {
	bind.HTMLGetter
	tag.TagGetter
	jaws.ClickHandler
	jaws.ContextMenuHandler
	jaws.InitialHTMLAttrHandler

	// Clicked returns an [Object] that will call fn when [jaws.ClickHandler.JawsClick] is invoked.
	Clicked(fn ClickedHook) (newobj Object)

	// ContextMenu returns an [Object] that will call fn when
	// [jaws.ContextMenuHandler.JawsContextMenu] is invoked.
	ContextMenu(fn ContextMenuHook) (newobj Object)

	// InitialHTMLAttr returns an [Object] that will call fn when
	// [jaws.InitialHTMLAttrHandler.JawsInitialHTMLAttr] is invoked.
	InitialHTMLAttr(fn InitialHTMLAttrHook) (newobj Object)
}

var _ Object = &object{}

type object struct {
	prev    *object
	handler any
}

func (obj *object) Clicked(fn ClickedHook) Object {
	return &object{
		prev:    obj,
		handler: fn,
	}
}

func (obj *object) ContextMenu(fn ContextMenuHook) Object {
	return &object{
		prev:    obj,
		handler: fn,
	}
}

func (obj *object) InitialHTMLAttr(fn InitialHTMLAttrHook) Object {
	return &object{
		prev:    obj,
		handler: fn,
	}
}

func (obj *object) JawsGetHTML(elem *jaws.Element) (html template.HTML) {
	for obj != nil {
		if h, ok := obj.handler.(bind.HTMLGetter); ok {
			html = h.JawsGetHTML(elem)
			break
		}
		obj = obj.prev
	}
	return
}

func (obj *object) JawsClick(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	for obj != nil {
		if fn, ok := obj.handler.(ClickedHook); ok {
			if err = fn(obj, elem, click); !errors.Is(err, jaws.ErrEventUnhandled) {
				break
			}
		}
		obj = obj.prev
	}
	return
}

func (obj *object) JawsContextMenu(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	for obj != nil {
		if fn, ok := obj.handler.(ContextMenuHook); ok {
			if err = fn(obj, elem, click); !errors.Is(err, jaws.ErrEventUnhandled) {
				break
			}
		}
		obj = obj.prev
	}
	return
}

func (obj *object) JawsInitialHTMLAttr(elem *jaws.Element) (attr template.HTMLAttr) {
	for obj != nil {
		if fn, ok := obj.handler.(InitialHTMLAttrHook); ok {
			if s := fn(obj, elem); s != "" {
				if attr != "" {
					attr += " "
				}
				attr += s
			}
		}
		obj = obj.prev
	}
	return
}

func (obj *object) JawsGetTag(ctx tag.Context) any {
	var tags []any
	for obj != nil {
		if h, ok := obj.handler.(tag.TagGetter); ok {
			if t := h.JawsGetTag(ctx); t != nil {
				tags = append(tags, t)
			}
		}
		obj = obj.prev
	}
	switch len(tags) {
	case 0:
		return nil
	case 1:
		return tags[0]
	}
	return tags
}

// New returns a new [Object] that renders innerHTML.
//
// innerHTML is passed to [bind.MakeHTMLGetter], which may or may not provide
// tags. Plain strings are trusted HTML.
func New(innerHTML any) (obj Object) {
	return &object{
		handler: bind.MakeHTMLGetter(innerHTML),
	}
}
