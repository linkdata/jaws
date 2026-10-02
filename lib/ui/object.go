package ui

import (
	"errors"
	"html/template"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/tag"
)

// ObjectClickedHook handles a click event for an [Object].
//
// obj is the Object on which JawsClick was invoked.
//
// Unlike [bind.ClickedHook], ObjectClickedHook receives an [Object] rather than
// a [bind.Binder].
type ObjectClickedHook func(obj Object, elem *jaws.Element, click jaws.Click) (err error)

// ObjectContextMenuHook handles a context menu event for an [Object].
//
// obj is the Object on which JawsContextMenu was invoked.
type ObjectContextMenuHook func(obj Object, elem *jaws.Element, click jaws.Click) (err error)

// ObjectInitialHTMLAttrHook provides attributes when an [Object] is initially rendered.
//
// obj is the Object on which JawsInitialHTMLAttr was invoked.
//
// ObjectInitialHTMLAttrHook is a type alias so a value of a defined function type
// with this signature can be passed to [Object.InitialHTMLAttr] without explicit
// conversion.
type ObjectInitialHTMLAttrHook = func(obj Object, elem *jaws.Element) (s template.HTMLAttr)

// Object is a chainable UI object.
//
// Each call to [Object.Clicked], [Object.ContextMenu], or
// [Object.InitialHTMLAttr] returns a new chain node wrapping its receiver. Calls
// to [jaws.ClickHandler.JawsClick] and
// [jaws.ContextMenuHandler.JawsContextMenu] run the corresponding hooks from
// newest to oldest. Dispatch continues while the result matches
// [jaws.ErrEventUnhandled] according to [errors.Is], including when wrapped,
// and stops at the first other result. If no hook handles the event, the invoked
// method returns an error matching [jaws.ErrEventUnhandled].
//
// [jaws.InitialHTMLAttrHandler.JawsInitialHTMLAttr] runs all initial-attribute
// hooks from newest to oldest. Their non-empty results are joined in that order
// with one space inserted between results.
//
// Every event and initial-attribute hook receives the Object on which dispatch
// was invoked. That Object may include links added after the hook.
//
// The adapted innerHTML supplies the Object's tag. Adding hooks preserves it.
// The tag remains subject to [tag.TagGetter]'s initialization, stability, and
// concurrency requirements. Use [tag.TagExpand] to obtain flattened, validated tags.
type Object interface {
	bind.HTMLGetter
	tag.TagGetter
	jaws.ClickHandler
	jaws.ContextMenuHandler
	jaws.InitialHTMLAttrHandler

	// Clicked adds fn as the newest click hook and returns the resulting [Object].
	Clicked(fn ObjectClickedHook) (newobj Object)

	// ContextMenu adds fn as the newest context-menu hook and returns the
	// resulting [Object].
	ContextMenu(fn ObjectContextMenuHook) (newobj Object)

	// InitialHTMLAttr adds fn as the newest initial-attribute hook and returns the
	// resulting [Object].
	InitialHTMLAttr(fn ObjectInitialHTMLAttrHook) (newobj Object)
}

var _ Object = &object{}

type object struct {
	prev *object
	// The defined hook types distinguish click from context-menu callbacks.
	handler any
}

func (obj *object) Clicked(fn ObjectClickedHook) Object {
	return &object{
		prev:    obj,
		handler: fn,
	}
}

func (obj *object) ContextMenu(fn ObjectContextMenuHook) Object {
	return &object{
		prev:    obj,
		handler: fn,
	}
}

func (obj *object) InitialHTMLAttr(fn ObjectInitialHTMLAttrHook) Object {
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
	invoked := obj
	err = jaws.ErrEventUnhandled
	for obj != nil {
		if fn, ok := obj.handler.(ObjectClickedHook); ok {
			if err = fn(invoked, elem, click); !errors.Is(err, jaws.ErrEventUnhandled) {
				break
			}
		}
		obj = obj.prev
	}
	return
}

func (obj *object) JawsContextMenu(elem *jaws.Element, click jaws.Click) (err error) {
	invoked := obj
	err = jaws.ErrEventUnhandled
	for obj != nil {
		if fn, ok := obj.handler.(ObjectContextMenuHook); ok {
			if err = fn(invoked, elem, click); !errors.Is(err, jaws.ErrEventUnhandled) {
				break
			}
		}
		obj = obj.prev
	}
	return
}

func (obj *object) JawsInitialHTMLAttr(elem *jaws.Element) (attr template.HTMLAttr) {
	invoked := obj
	for obj != nil {
		if fn, ok := obj.handler.(ObjectInitialHTMLAttrHook); ok {
			if s := fn(invoked, elem); s != "" {
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

// JawsGetTag returns the adapted innerHTML's tag.
func (obj *object) JawsGetTag() any {
	for obj != nil && obj.prev != nil {
		obj = obj.prev
	}
	if obj != nil {
		if h, ok := obj.handler.(tag.TagGetter); ok {
			return h.JawsGetTag()
		}
	}
	return nil
}

// New returns a new [Object] that renders innerHTML.
//
// innerHTML is passed to [bind.MakeHTMLGetter]. Its HTML and any tags exposed by
// the adapted value's [tag.TagGetter] are used. An existing [bind.HTMLGetter]
// without JawsGetTag contributes no tag. innerHTML's event and initial-attribute
// methods are not inherited; add those behaviors with [Object.Clicked],
// [Object.ContextMenu], and [Object.InitialHTMLAttr]. Plain strings are trusted
// HTML.
func New(innerHTML any) (obj Object) {
	return &object{
		handler: bind.MakeHTMLGetter(innerHTML),
	}
}
