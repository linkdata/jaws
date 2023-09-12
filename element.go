package jaws

import (
	"fmt"
	"html"
	"html/template"
	"strconv"
	"sync/atomic"

	"github.com/linkdata/jaws/what"
)

// An Element is an instance of an UI object and it's user data in a Request.
type Element struct {
	ui       UI            // (read-only) the UI object
	jid      Jid           // (read-only) JaWS ID, unique to this Element within it's Request
	*Request               // (read-only) the Request the Element belongs to
	dirty    uint64        // (atomic) if not zero, needs Update() to be called
	Data     []interface{} // the optional data provided to the Request.UI() call
}

func (e *Element) String() string {
	return fmt.Sprintf("Element{%T, id=%q, Tags: %v}", e.ui, e.jid, e.Tags())
}

func (e *Element) Tags() []interface{} {
	return e.TagsOf(e)
}

// Jid returns the JaWS ID for this element, unique within it's Request.
func (e *Element) Jid() Jid {
	return e.jid
}

// AppendAttrs appends strings present in Data for this element, prepended by spaces.
func (e *Element) AppendAttrs(b []byte) []byte {
	for _, v := range e.Data {
		if s, ok := v.(string); ok {
			b = append(b, ' ')
			b = append(b, s...)
		}
	}
	return b
}

// Attrs returns the strings present in Data for this element.
func (e *Element) Attrs() (attrs []string) {
	for _, v := range e.Data {
		if s, ok := v.(string); ok {
			attrs = append(attrs, s)
		}
	}
	return
}

// UI returns the UI object.
func (e *Element) UI() UI {
	return e.ui
}

// Dirty marks the Element as needing UI().JawsUpdate() to be called.
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
		panic(fmt.Sprintf("jaws: don't know how to render %T as template.HTML", v))
	}
	return template.HTML(html.EscapeString(s))
}
