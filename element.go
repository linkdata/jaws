package jaws

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"
	"sync/atomic"

	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

// Element is an instance of a [Request], a [UI] object and a [Jid].
type Element struct {
	*Request // (read-only) the Request the Element belongs to
	// internals
	ui       UI          // the UI object
	handlers []any       // custom handlers registered, if any
	jid      jid.Jid     // JaWS ID, unique to this Element within its Request
	deleted  atomic.Bool // true if deleteElement() has been called for this Element
}

func (elem *Element) String() string {
	return fmt.Sprintf("Element{%T, id=%q, Tags: %v}", elem.Ui(), elem.Jid(), elem.Request.TagsOf(elem))
}

// AddHandlers adds the given handlers to the [Element].
func (elem *Element) AddHandlers(h ...any) {
	if !elem.deleted.Load() {
		elem.handlers = append(elem.handlers, h...)
	}
}

// Tag adds the given tags to the [Element].
func (elem *Element) Tag(tags ...any) {
	if !elem.deleted.Load() {
		elem.Request.Tag(elem, tags...)
	}
}

// HasTag returns true if this Element has the given tag.
func (elem *Element) HasTag(tagValue any) bool {
	return !elem.deleted.Load() && elem.Request.HasTag(elem, tagValue)
}

// Jid returns the JaWS ID for this [Element], unique within its [Request].
func (elem *Element) Jid() jid.Jid {
	return elem.jid
}

// Ui returns the [UI] object.
func (elem *Element) Ui() UI {
	return elem.ui
}

func (elem *Element) maybeDirty(tagValue any, err error) (bool, error) {
	switch err {
	case nil:
		elem.Dirty(tagValue)
		return true, nil
	case ErrValueUnchanged:
		return false, nil
	}
	return false, err
}

func (elem *Element) renderDebug(w io.Writer) {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "<!-- id=%q %T tags=[", elem.Jid(), elem.Ui())
	if elem.mu.TryRLock() {
		defer elem.mu.RUnlock()
		for i, tagValue := range elem.tagsOfLocked(elem) {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(tag.TagString(tagValue))
		}
	} else {
		sb.WriteString("n/a")
	}
	sb.WriteByte(']')
	_, _ = w.Write([]byte(strings.ReplaceAll(sb.String(), "-->", "==>") + " -->"))
}

// JawsRender calls [Renderer.JawsRender] for this [Element].
//
// Do not call this yourself unless it is from within another JawsRender implementation.
func (elem *Element) JawsRender(w io.Writer, params []any) (err error) {
	if !elem.deleted.Load() {
		if err = elem.Ui().JawsRender(elem, w, params); err == nil {
			if elem.Jaws.Debug {
				elem.renderDebug(w)
			}
		}
	}
	return
}

// JawsUpdate calls [Updater.JawsUpdate] for this [Element].
//
// Do not call this yourself unless it is from within another JawsUpdate implementation.
func (elem *Element) JawsUpdate() {
	if !elem.deleted.Load() {
		elem.Ui().JawsUpdate(elem)
	}
}

func (elem *Element) queue(wht what.What, data string) {
	if !elem.deleted.Load() {
		elem.Request.queue(wire.WsMsg{
			Data: data,
			Jid:  elem.jid,
			What: wht,
		})
	}
}

// SetAttr queues sending a new attribute value
// to the browser for the [Element] with the given JaWS ID in this [Request].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) SetAttr(attr, value string) {
	elem.queue(what.SAttr, attr+"\n"+value)
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the [Element] with the given JaWS ID in this [Request].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) RemoveAttr(attr string) {
	elem.queue(what.RAttr, attr)
}

// SetClass queues sending a class
// to the browser for the [Element] with the given JaWS ID in this [Request].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) SetClass(cls string) {
	elem.queue(what.SClass, cls)
}

// RemoveClass queues sending a request to remove a class
// to the browser for the [Element] with the given JaWS ID in this [Request].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) RemoveClass(cls string) {
	elem.queue(what.RClass, cls)
}

// SetInner queues sending new inner HTML content
// to the browser for the [Element].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) SetInner(innerHTML template.HTML) {
	elem.queue(what.Inner, string(innerHTML))
}

// SetValue queues sending a new current input value in textual form
// to the browser for the [Element] with the given JaWS ID in this [Request].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) SetValue(value string) {
	elem.queue(what.Value, value)
}

// Replace replaces the [Element]'s entire HTML DOM node with new HTML code.
// If the HTML code doesn't seem to contain correct HTML ID, it panics.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) Replace(htmlCode template.HTML) {
	if !elem.deleted.Load() {
		var b []byte
		b = append(b, "id="...)
		b = elem.Jid().AppendQuote(b)
		if !bytes.Contains([]byte(htmlCode), b) {
			panic(errors.New("jaws: Element.Replace(): expected HTML " + string(b)))
		}
		elem.queue(what.Replace, string(htmlCode))
	}
}

// Append appends a new HTML element as a child to the current one.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) Append(htmlCode template.HTML) {
	elem.queue(what.Append, string(htmlCode))
}

// Order reorders the HTML elements.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) Order(jidList []jid.Jid) {
	if !elem.deleted.Load() && len(jidList) > 0 {
		var b []byte
		for i, jid := range jidList {
			if i > 0 {
				b = append(b, ' ')
			}
			b = jid.Append(b)
		}
		elem.queue(what.Order, string(b))
	}
}

// Remove requests that the HTML child with the given HTML ID of this [Element]
// is removed from the [Request] and its HTML element from the browser.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (elem *Element) Remove(htmlID string) {
	elem.queue(what.Remove, htmlID)
}

// ApplyParams parses the parameters passed to UI() when creating a new [Element],
// adding UI tags, adding any additional event handlers found.
//
// Returns the list of HTML attributes found, if any.
func (elem *Element) ApplyParams(params []any) (attrs []template.HTMLAttr) {
	tags, handlers, rawAttrs := ParseParams(params)
	if !elem.deleted.Load() {
		elem.handlers = append(elem.handlers, handlers...)
		elem.Tag(tags...)
		for _, s := range rawAttrs {
			attr := template.HTMLAttr(s) // #nosec G203
			attrs = append(attrs, attr)
		}
	}
	return
}

// ApplyGetter examines getter, and if it is not nil, either adds it
// as a tag, or, if it is a [tag.TagGetter], adds the result of that as a tag.
//
// If getter is an [InputHandler], [ClickHandler], [ContextMenuHandler] or
// [InitialHTMLAttrHandler], relevant values are added to the [Element].
//
// Finally, if getter is an [InitHandler], its JawsInit
// function is called.
//
// Returns the Tag(s) added (or nil if getter was nil), any initial HTML attrs
// provided by InitialHTMLAttrHandler, and any error returned from JawsInit()
// if it was called.
func (elem *Element) ApplyGetter(getter any) (tagValue any, attrs []template.HTMLAttr, err error) {
	if getter != nil {
		tagValue = getter
		if tagger, ok := getter.(tag.TagGetter); ok {
			tagValue = tagger.JawsGetTag(elem.Request)
		}
		if _, ok := getter.(InputHandler); ok {
			elem.handlers = append(elem.handlers, getter)
		} else if _, ok := getter.(ClickHandler); ok {
			elem.handlers = append(elem.handlers, getter)
		} else if _, ok := getter.(ContextMenuHandler); ok {
			elem.handlers = append(elem.handlers, getter)
		}
		if ah, ok := getter.(InitialHTMLAttrHandler); ok {
			if attr := ah.JawsInitialHTMLAttr(elem); attr != "" {
				attrs = append(attrs, attr)
			}
		}
		elem.Tag(tagValue)
		if initer, ok := getter.(InitHandler); ok {
			err = initer.JawsInit(elem)
		}
	}
	return
}
