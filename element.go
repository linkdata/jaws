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

func (e *Element) String() string {
	return fmt.Sprintf("Element{%T, id=%q, Tags: %v}", e.Ui(), e.Jid(), e.Request.TagsOf(e))
}

// AddHandlers adds the given handlers to the [Element].
func (e *Element) AddHandlers(h ...any) {
	if !e.deleted.Load() {
		e.handlers = append(e.handlers, h...)
	}
}

// Tag adds the given tags to the [Element].
func (e *Element) Tag(tags ...any) {
	if !e.deleted.Load() {
		e.Request.Tag(e, tags...)
	}
}

// HasTag returns true if this Element has the given tag.
func (e *Element) HasTag(tagValue any) bool {
	return !e.deleted.Load() && e.Request.HasTag(e, tagValue)
}

// Jid returns the JaWS ID for this [Element], unique within its [Request].
func (e *Element) Jid() jid.Jid {
	return e.jid
}

// Ui returns the [UI] object.
func (e *Element) Ui() UI {
	return e.ui
}

func (e *Element) maybeDirty(tagValue any, err error) (bool, error) {
	switch err {
	case nil:
		e.Dirty(tagValue)
		return true, nil
	case ErrValueUnchanged:
		return false, nil
	}
	return false, err
}

func (e *Element) renderDebug(w io.Writer) {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "<!-- id=%q %T tags=[", e.Jid(), e.Ui())
	if e.mu.TryRLock() {
		defer e.mu.RUnlock()
		for i, tagValue := range e.tagsOfLocked(e) {
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
func (e *Element) JawsRender(w io.Writer, params []any) (err error) {
	if !e.deleted.Load() {
		if err = e.Ui().JawsRender(e, w, params); err == nil {
			if e.Jaws.Debug {
				e.renderDebug(w)
			}
		}
	}
	return
}

// JawsUpdate calls [Updater.JawsUpdate] for this [Element].
//
// Do not call this yourself unless it is from within another JawsUpdate implementation.
func (e *Element) JawsUpdate() {
	if !e.deleted.Load() {
		e.Ui().JawsUpdate(e)
	}
}

func (e *Element) queue(wht what.What, data string) {
	if !e.deleted.Load() {
		e.Request.queue(wire.WsMsg{
			Data: data,
			Jid:  e.jid,
			What: wht,
		})
	}
}

// SetAttr queues sending a new attribute value
// to the browser for the [Element] with the given JaWS ID in this [Request].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) SetAttr(attr, val string) {
	e.queue(what.SAttr, attr+"\n"+val)
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the [Element] with the given JaWS ID in this [Request].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) RemoveAttr(attr string) {
	e.queue(what.RAttr, attr)
}

// SetClass queues sending a class
// to the browser for the [Element] with the given JaWS ID in this [Request].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) SetClass(cls string) {
	e.queue(what.SClass, cls)
}

// RemoveClass queues sending a request to remove a class
// to the browser for the [Element] with the given JaWS ID in this [Request].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) RemoveClass(cls string) {
	e.queue(what.RClass, cls)
}

// SetInner queues sending new inner HTML content
// to the browser for the [Element].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) SetInner(innerHTML template.HTML) {
	e.queue(what.Inner, string(innerHTML))
}

// SetValue queues sending a new current input value in textual form
// to the browser for the [Element] with the given JaWS ID in this [Request].
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) SetValue(val string) {
	e.queue(what.Value, val)
}

// Replace replaces the [Element]'s entire HTML DOM node with new HTML code.
// If the HTML code doesn't seem to contain correct HTML ID, it panics.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) Replace(htmlCode template.HTML) {
	if !e.deleted.Load() {
		var b []byte
		b = append(b, "id="...)
		b = e.Jid().AppendQuote(b)
		if !bytes.Contains([]byte(htmlCode), b) {
			panic(errors.New("jaws: Element.Replace(): expected HTML " + string(b)))
		}
		e.queue(what.Replace, string(htmlCode))
	}
}

// Append appends a new HTML element as a child to the current one.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) Append(htmlCode template.HTML) {
	e.queue(what.Append, string(htmlCode))
}

// Order reorders the HTML elements.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) Order(jidList []jid.Jid) {
	if !e.deleted.Load() && len(jidList) > 0 {
		var b []byte
		for i, jid := range jidList {
			if i > 0 {
				b = append(b, ' ')
			}
			b = jid.Append(b)
		}
		e.queue(what.Order, string(b))
	}
}

// Remove requests that the HTML child with the given HTML ID of this [Element]
// is removed from the [Request] and its HTML element from the browser.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) Remove(htmlId string) {
	e.queue(what.Remove, htmlId)
}

// ApplyParams parses the parameters passed to UI() when creating a new [Element],
// adding UI tags, adding any additional event handlers found.
//
// Returns the list of HTML attributes found, if any.
func (e *Element) ApplyParams(params []any) (retv []template.HTMLAttr) {
	tags, handlers, attrs := ParseParams(params)
	if !e.deleted.Load() {
		e.handlers = append(e.handlers, handlers...)
		e.Tag(tags...)
		for _, s := range attrs {
			attr := template.HTMLAttr(s) // #nosec G203
			retv = append(retv, attr)
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
func (e *Element) ApplyGetter(getter any) (tagValue any, attrs []template.HTMLAttr, err error) {
	if getter != nil {
		tagValue = getter
		if tagger, ok := getter.(tag.TagGetter); ok {
			tagValue = tagger.JawsGetTag(e.Request)
		}
		if _, ok := getter.(InputHandler); ok {
			e.handlers = append(e.handlers, getter)
		} else if _, ok := getter.(ClickHandler); ok {
			e.handlers = append(e.handlers, getter)
		} else if _, ok := getter.(ContextMenuHandler); ok {
			e.handlers = append(e.handlers, getter)
		}
		if ah, ok := getter.(InitialHTMLAttrHandler); ok {
			if attr := ah.JawsInitialHTMLAttr(e); attr != "" {
				attrs = append(attrs, attr)
			}
		}
		e.Tag(tagValue)
		if initer, ok := getter.(InitHandler); ok {
			err = initer.JawsInit(e)
		}
	}
	return
}
