package jaws

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"io"
	"strconv"
	"sync/atomic"

	"github.com/linkdata/jaws/what"
)

// An Element is an instance of a *Request, an UI object and a Jid.
type Element struct {
	ui       UI   // (read-only) the UI object
	jid      Jid  // (read-only) JaWS ID, unique to this Element within it's Request
	updating bool // about to have Update() called
	*Request      // (read-only) the Request the Element belongs to
}

func (e *Element) String() string {
	return fmt.Sprintf("Element{%T, id=%q, Tags: %v}", e.ui, e.jid, e.Request.TagsOf(e))
}

// Tag adds the given tags to the Element.
func (e *Element) Tag(tags ...interface{}) {
	e.Request.Tag(e, tags...)
}

// HasTag returns true if this Element has the given tag.
func (e *Element) HasTag(tag interface{}) bool {
	return e.Request.HasTag(e, tag)
}

// Jid returns the JaWS ID for this Element, unique within it's Request.
func (e *Element) Jid() Jid {
	return e.jid
}

// Ui returns the UI object.
func (e *Element) Ui() UI {
	return e.ui
}

// Dirty marks this Element (only) as needing UI().JawsUpdate() to be called.
func (e *Element) Dirty() {
	if e != nil {
		e.Request.appendDirtyTags(e)
	}
}

// Render calls UI().JawsRender() for this Element.
func (e *Element) Render(w io.Writer, params []interface{}) {
	e.ui.JawsRender(e, w, params)
}

func (e *Element) ToHtml(val interface{}) template.HTML {
	var s string
	switch v := val.(type) {
	case string:
		s = v
	case template.HTML:
		return v
	case *atomic.Value:
		return e.ToHtml(v.Load())
	case fmt.Stringer:
		s = v.String()
	case float64:
		s = strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		s = strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		s = strconv.Itoa(v)
	default:
		panic(fmt.Errorf("jaws: don't know how to render %T as template.HTML", v))
	}
	return template.HTML(html.EscapeString(s))
}

func (e *Element) send(wht what.What, data string) {
	e.Request.send(wsMsg{
		Data: data,
		Jid:  e.jid,
		What: wht,
	})
}

// SetAttr queues sending a new attribute value
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) SetAttr(attr, val string) {
	e.send(what.SAttr, attr+"\n"+val)
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) RemoveAttr(attr string) {
	e.send(what.RAttr, attr)
}

// SetClass a queues sending a class
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) SetClass(cls string) {
	e.send(what.SClass, cls)
}

// RemoveClass queues sending a request to remove a class
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) RemoveClass(cls string) {
	e.send(what.RClass, cls)
}

// SetInner queues sending a new inner HTML content
// to the browser for the Element.
func (e *Element) SetInner(innerHtml template.HTML) {
	e.send(what.Inner, string(innerHtml))
}

// SetValue queues sending a new current input value in textual form
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) SetValue(val string) {
	e.send(what.Value, val)
}

// Replace replaces the elements entire HTML DOM node with new HTML code.
// If the HTML code doesn't seem to contain correct HTML ID, it panics.
func (e *Element) Replace(htmlCode template.HTML) {
	var b []byte
	b = append(b, "id="...)
	b = e.Jid().AppendQuote(b)
	if !bytes.Contains([]byte(htmlCode), b) {
		panic(fmt.Errorf("jaws: Element.Replace(): expected HTML " + string(b)))
	}
	e.send(what.Replace, string(htmlCode))
}

// Append appends a new HTML element as a child to the current one.
func (e *Element) Append(htmlCode template.HTML) {
	e.send(what.Append, string(htmlCode))
}

// Order reorders the HTML child elements of the current Element.
func (e *Element) Order(jidList []Jid) {
	if len(jidList) > 0 {
		var b []byte
		for i, jid := range jidList {
			if i > 0 {
				b = append(b, ' ')
			}
			b = jid.AppendInt(b)
		}
		e.send(what.Order, string(b))
	}
}

// Remove requests that this Element is removed from the Request and it's HTML element from the browser.
func (e *Element) Remove() {
	e.Request.remove(e)
}
