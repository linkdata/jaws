package jaws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
)

// An Element is an instance of a *Request, an UI object and a Jid.
type Element struct {
	*Request // (read-only) the Request the Element belongs to
	// internals
	ui       UI             // the UI object
	handlers []EventHandler // custom event handlers registered, if any
	jid      jid.Jid        // JaWS ID, unique to this Element within it's Request
	deleted  bool           // true if deleteElement() has been called for this Element
}

func (e *Element) String() string {
	return fmt.Sprintf("Element{%T, id=%q, Tags: %v}", e.Ui(), e.Jid(), e.Request.TagsOf(e))
}

// Tag adds the given tags to the Element.
func (e *Element) Tag(tags ...any) {
	if !e.deleted {
		e.Request.Tag(e, tags...)
	}
}

// HasTag returns true if this Element has the given tag.
func (e *Element) HasTag(tag any) bool {
	return !e.deleted && e.Request.HasTag(e, tag)
}

// Jid returns the JaWS ID for this Element, unique within it's Request.
func (e *Element) Jid() jid.Jid {
	return e.jid
}

// Ui returns the UI object.
func (e *Element) Ui() UI {
	return e.ui
}

func (e *Element) maybeDirty(tag any, err error) (bool, error) {
	switch err {
	case nil:
		e.Dirty(tag)
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
		for i, tag := range e.tagsOfLocked(e) {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(TagString(tag))
		}
	} else {
		sb.WriteString("n/a")
	}
	sb.WriteByte(']')
	_, _ = w.Write([]byte(strings.ReplaceAll(sb.String(), "-->", "==>") + " -->"))
}

// JawsRender calls Ui().JawsRender() for this Element.
//
// Do not call this yourself unless it's from within another JawsRender implementation.
func (e *Element) JawsRender(w io.Writer, params []any) (err error) {
	if !e.deleted {
		if err = e.Ui().JawsRender(e, w, params); err == nil {
			if e.Jaws.Debug {
				e.renderDebug(w)
			}
		}
	}
	return
}

// JawsUpdate calls Ui().JawsUpdate() for this Element.
//
// Do not call this yourself unless it's from within another JawsUpdate implementation.
func (e *Element) JawsUpdate() {
	if !e.deleted {
		e.Ui().JawsUpdate(e)
	}
}

func (e *Element) queue(wht what.What, data string) {
	if !e.deleted {
		e.Request.queue(wsMsg{
			Data: data,
			Jid:  e.jid,
			What: wht,
		})
	}
}

// SetAttr queues sending a new attribute value
// to the browser for the Element with the given JaWS ID in this Request.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) SetAttr(attr, val string) {
	e.queue(what.SAttr, attr+"\n"+val)
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the Element with the given JaWS ID in this Request.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) RemoveAttr(attr string) {
	e.queue(what.RAttr, attr)
}

// SetClass a queues sending a class
// to the browser for the Element with the given JaWS ID in this Request.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) SetClass(cls string) {
	e.queue(what.SClass, cls)
}

// RemoveClass queues sending a request to remove a class
// to the browser for the Element with the given JaWS ID in this Request.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) RemoveClass(cls string) {
	e.queue(what.RClass, cls)
}

// SetInner queues sending a new inner HTML content
// to the browser for the Element.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) SetInner(innerHtml template.HTML) {
	e.queue(what.Inner, string(innerHtml))
}

// SetValue queues sending a new current input value in textual form
// to the browser for the Element with the given JaWS ID in this Request.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) SetValue(val string) {
	e.queue(what.Value, val)
}

// Replace replaces the elements entire HTML DOM node with new HTML code.
// If the HTML code doesn't seem to contain correct HTML ID, it panics.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) Replace(htmlCode template.HTML) {
	if !e.deleted {
		var b []byte
		b = append(b, "id="...)
		b = e.Jid().AppendQuote(b)
		if !bytes.Contains([]byte(htmlCode), b) {
			panic(fmt.Errorf("jaws: Element.Replace(): expected HTML " + string(b)))
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
	if !e.deleted && len(jidList) > 0 {
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

// Remove requests that the HTML child with the given HTML ID of this Element
// is removed from the Request and it's HTML element from the browser.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *Element) Remove(htmlId string) {
	e.queue(what.Remove, htmlId)
}

func (e *Element) JsSet(val any) (err error) {
	var b []byte
	if b, err = json.Marshal(val); err == nil {
		e.queue(what.Set, string(b))
	}
	return
}

// ApplyParams parses the parameters passed to UI() when creating a new Element,
// adding UI tags, adding any additional event handlers found.
//
// Returns the list of HTML attributes found, if any.
func (e *Element) ApplyParams(params []any) (retv []template.HTMLAttr) {
	tags, handlers, attrs := ParseParams(params)
	if !e.deleted {
		e.handlers = append(e.handlers, handlers...)
		e.Tag(tags...)
		for _, s := range attrs {
			attr := template.HTMLAttr(s) // #nosec G203
			retv = append(retv, attr)
		}
	}
	return
}

// ApplyGetter examines getter, and if it's not nil, either adds it
// as a Tag, or, if it is a TagGetter, adds the result of that as a Tag.
// If getter is a ClickHandler or an EventHandler, it's added to the
// list of handlers for the Element.
//
// Returns the Tag added, or nil if getter was nil.
func (e *Element) ApplyGetter(getter any) (tag any) {
	if getter != nil {
		tag = getter
		if tagger, ok := getter.(TagGetter); ok {
			tag = tagger.JawsGetTag(e.Request)
		}
		e.Tag(tag)
		if ch, ok := getter.(ClickHandler); ok {
			e.handlers = append(e.handlers, clickHandlerWapper{ch})
		}
		if eh, ok := getter.(EventHandler); ok {
			e.handlers = append(e.handlers, eh)
		}
	}
	return
}
