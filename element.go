package jaws

import (
	"bytes"
	"fmt"
	"html/template"
	"io"

	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
)

// An Element is an instance of a *Request, an UI object and a Jid.
type Element struct {
	*Request // (read-only) the Request the Element belongs to
	// internals
	ui       UI             // the UI object
	wsQueue  []wsMsg        // changes queued
	handlers []EventHandler // custom event handlers registered, if any
	jid      jid.Jid        // JaWS ID, unique to this Element within it's Request
	updating bool           // about to have Update() called
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

// Update calls Ui().JawsUpdate() for this Element.
func (e *Element) Update() {
	if !e.deleted {
		e.Ui().JawsUpdate(e)
	}
}

// Render calls Request.JawsRender() for this Element.
func (e *Element) Render(w io.Writer, params []any) (err error) {
	if !e.deleted {
		err = e.Request.JawsRender(e, w, params)
	}
	return
}

func (e *Element) queue(wht what.What, data string) {
	if !e.deleted {
		if len(e.wsQueue) < maxWsQueueLengthPerElement {
			e.wsQueue = append(e.wsQueue, wsMsg{
				Data: data,
				Jid:  e.jid,
				What: wht,
			})
		} else {
			e.Request.cancel(ErrWebsocketQueueOverflow)
		}
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

// ParseParams parses the parameters passed to UI() when creating a new Element,
// setting event handlers and returning a list of HTML attributes.
func (e *Element) ParseParams(params []any) (attrs []template.HTMLAttr) {
	if !e.deleted {
		for i := range params {
			switch data := params[i].(type) {
			case template.HTMLAttr:
				attrs = append(attrs, data)
			case []template.HTMLAttr:
				attrs = append(attrs, data...)
			case string:
				attrs = append(attrs, template.HTMLAttr(data)) // #nosec G203
			case []string:
				for _, s := range data {
					attrs = append(attrs, template.HTMLAttr(s)) // #nosec G203
				}
			case EventFn:
				if data != nil {
					e.handlers = append(e.handlers, eventFnWrapper{data})
				}
			default:
				if h, ok := data.(ClickHandler); ok {
					e.handlers = append(e.handlers, clickHandlerWapper{h})
				}
				if h, ok := data.(EventHandler); ok {
					e.handlers = append(e.handlers, h)
				}
				e.Tag(data)
			}
		}
	}
	return
}
