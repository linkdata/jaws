package jaws

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
)

type Element interface {
	fmt.Stringer
	Request
	GetRequest() Request
	Tag(tags ...any)
	HasTag(tag any) bool
	Jid() jid.Jid
	Ui() UI
	JawsRender(w io.Writer, params []any) (err error)
	JawsUpdate()
	SetAttr(attr, val string)
	RemoveAttr(attr string)
	SetClass(cls string)
	RemoveClass(cls string)
	SetInner(innerHTML template.HTML)
	SetValue(val string)
	Replace(htmlCode template.HTML)
	Append(htmlCode template.HTML)
	Order(jidList []jid.Jid)
	Remove(htmlId string)
	ApplyParams(params []any) (retv []template.HTMLAttr)
	ApplyGetter(getter any) (tag any, err error)
	MaybeDirty(tag any, err error) (bool, error)
	RenderDebug(w io.Writer)
	IsDeleted() bool
	SetDeleted()
	AddHandlers(h ...EventHandler)
	GetHandlers() []EventHandler
}

var _ Element = &element{}

// An element is an instance of a *Request, an UI object and a Jid.
type element struct {
	Request // (read-only) the Request the Element belongs to
	// internals
	ui       UI             // the UI object
	handlers []EventHandler // custom event handlers registered, if any
	jid      jid.Jid        // JaWS ID, unique to this Element within it's Request
	deleted  bool           // true if deleteElement() has been called for this Element
}

func (e *element) String() string {
	return fmt.Sprintf("Element{%T, id=%q, Tags: %v}", e.Ui(), e.Jid(), e.GetRequest().TagsOf(e))
}

func (e *element) GetRequest() Request {
	return e.Request
}

func (e *element) AddHandlers(h ...EventHandler) {
	e.handlers = append(e.handlers, h...)
}

func (e *element) GetHandlers() []EventHandler {
	return e.handlers
}

// Tag adds the given tags to the Element.
func (e *element) Tag(tags ...any) {
	if !e.deleted {
		e.Request.ElementSetTag(e, tags...)
	}
}

// HasTag returns true if this Element has the given tag.
func (e *element) HasTag(tag any) bool {
	return !e.deleted && e.Request.ElementHasTag(e, tag)
}

// Jid returns the JaWS ID for this Element, unique within it's Request.
func (e *element) Jid() jid.Jid {
	return e.jid
}

// Ui returns the UI object.
func (e *element) Ui() UI {
	return e.ui
}

func (e *element) IsDeleted() bool {
	return e.deleted
}

func (e *element) SetDeleted() {
	e.deleted = true
}

func (e *element) MaybeDirty(tag any, err error) (bool, error) {
	switch err {
	case nil:
		e.Dirty(tag)
		return true, nil
	case ErrValueUnchanged:
		return false, nil
	}
	return false, err
}

func (e *element) RenderDebug(w io.Writer) {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "<!-- id=%q %T tags=[", e.Jid(), e.Ui())
	if rq, ok := e.Request.(*request); ok && rq.mu.TryRLock() {
		defer rq.mu.RUnlock()
		for i, tag := range e.TagsOfLocked(e) {
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
func (e *element) JawsRender(w io.Writer, params []any) (err error) {
	if !e.deleted {
		if err = e.Ui().JawsRender(e, w, params); err == nil {
			if e.Jaws().IsDebug() {
				e.RenderDebug(w)
			}
		}
	}
	return
}

// JawsUpdate calls Ui().JawsUpdate() for this Element.
//
// Do not call this yourself unless it's from within another JawsUpdate implementation.
func (e *element) JawsUpdate() {
	if !e.deleted {
		e.Ui().JawsUpdate(e)
	}
}

func (e *element) queue(wht what.What, data string) {
	if !e.deleted {
		e.Request.Queue(wsMsg{
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
func (e *element) SetAttr(attr, val string) {
	e.queue(what.SAttr, attr+"\n"+val)
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the Element with the given JaWS ID in this Request.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *element) RemoveAttr(attr string) {
	e.queue(what.RAttr, attr)
}

// SetClass a queues sending a class
// to the browser for the Element with the given JaWS ID in this Request.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *element) SetClass(cls string) {
	e.queue(what.SClass, cls)
}

// RemoveClass queues sending a request to remove a class
// to the browser for the Element with the given JaWS ID in this Request.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *element) RemoveClass(cls string) {
	e.queue(what.RClass, cls)
}

// SetInner queues sending a new inner HTML content
// to the browser for the Element.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *element) SetInner(innerHTML template.HTML) {
	e.queue(what.Inner, string(innerHTML))
}

// SetValue queues sending a new current input value in textual form
// to the browser for the Element with the given JaWS ID in this Request.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *element) SetValue(val string) {
	e.queue(what.Value, val)
}

// Replace replaces the elements entire HTML DOM node with new HTML code.
// If the HTML code doesn't seem to contain correct HTML ID, it panics.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *element) Replace(htmlCode template.HTML) {
	if !e.deleted {
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
func (e *element) Append(htmlCode template.HTML) {
	e.queue(what.Append, string(htmlCode))
}

// Order reorders the HTML elements.
//
// Call this only during JawsRender() or JawsUpdate() processing.
func (e *element) Order(jidList []jid.Jid) {
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
func (e *element) Remove(htmlId string) {
	e.queue(what.Remove, htmlId)
}

// ApplyParams parses the parameters passed to UI() when creating a new Element,
// adding UI tags, adding any additional event handlers found.
//
// Returns the list of HTML attributes found, if any.
func (e *element) ApplyParams(params []any) (retv []template.HTMLAttr) {
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
//
// If getter is a ClickHandler or an EventHandler, it's added to the
// list of handlers for the Element.
//
// Finally, if getter is an InitHandler, it's JawsInit()
// function is called.
//
// Returns the Tag added, or nil if getter was nil, along with
// any error returned from JawsInit() if it was called.
func (e *element) ApplyGetter(getter any) (tag any, err error) {
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
		if initer, ok := getter.(InitHandler); ok {
			err = initer.JawsInit(e)
		}
	}
	return
}
