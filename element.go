package jaws

import (
	"fmt"
	"html"
	"html/template"
	"strconv"
	"sync/atomic"

	"github.com/linkdata/jaws/what"
)

// An Element is an instance of a *Request, an UI object and a Jid.
type Element struct {
	ui       UI     // (read-only) the UI object
	jid      Jid    // (read-only) JaWS ID, unique to this Element within it's Request
	*Request        // (read-only) the Request the Element belongs to
	dirty    uint64 // (atomic) if not zero, needs Update() to be called
}

func (e *Element) String() string {
	return fmt.Sprintf("Element{%T, id=%q, Tags: %v}", e.ui, e.jid, e.Tags())
}

// Tag adds the given tags to the Element.
func (e *Element) Tag(tags ...interface{}) {
	e.Request.Tag(e, tags...)
}

// HasTag returns true if this Element has the given tag.
func (e *Element) HasTag(tag interface{}) bool {
	return e.Request.HasTag(e, tag)
}

func (e *Element) Tags() (tags []interface{}) {
	if tagger, ok := e.ui.(Tagger); ok {
		tags = tagger.JawsTags(e.Request, tags)
	}
	return
}

// Jid returns the JaWS ID for this Element, unique within it's Request.
func (e *Element) Jid() Jid {
	return e.jid
}

// UI returns the UI object.
func (e *Element) UI() UI {
	return e.ui
}

// Dirty marks this Element (only) as needing UI().JawsUpdate() to be called.
func (e *Element) Dirty() {
	atomic.StoreUint64(&e.dirty, atomic.AddUint64(&e.Request.dirty, 1))
}

func (e *Element) clearDirt() (dirt uint64) {
	if e != nil {
		dirt = atomic.SwapUint64(&e.dirty, 0)
	}
	return
}

// DirtyOthers marks all Elements except this one that have one or more of the given tags as dirty.
func (e *Element) DirtyOthers(tags ...interface{}) {
	for _, tag := range tags {
		e.Jaws.Broadcast(Message{
			Tag:  tag,
			What: what.Dirty,
			from: e,
		})
	}
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
