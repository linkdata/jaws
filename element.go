package jaws

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"strconv"
	"sync/atomic"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

type elemItem struct {
	name   string
	value  string
	remove bool
	dirty  bool
}

const elemReplaceMagic = ">R"
const elemInnerMagic = ">I"
const elemValueMagic = ">V"

// An Element is an instance of an UI object and it's user data in a Request.
type Element struct {
	ui       UI               // (read-only) the UI object
	jid      Jid              // (read-only) JaWS ID, unique to this Element within it's Request
	*Request                  // (read-only) the Request the Element belongs to
	dirty    uint64           // if not zero, needs Update() to be called
	Data     []interface{}    // the optional data provided to the Request.UI() call
	mu       deadlock.RWMutex // protects following
	items    []elemItem       // currently known items
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

func (e *Element) appendTodo(msgs []wsMsg) []wsMsg {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.items {
		if e.items[i].dirty {
			e.items[i].dirty = false
			if e.items[i].remove {
				e.items[i].remove = false
				msgs = append(msgs, wsMsg{
					Jid:  e.jid,
					Data: e.items[i].name,
					What: what.RAttr,
				})
				continue
			}
			switch e.items[i].name {
			case elemReplaceMagic:
				msgs = append(msgs, wsMsg{
					Jid:  e.jid,
					Data: e.items[i].value,
					What: what.Replace,
				})
			case elemInnerMagic:
				msgs = append(msgs, wsMsg{
					Jid:  e.jid,
					Data: e.items[i].value,
					What: what.Inner,
				})
			case elemValueMagic:
				msgs = append(msgs, wsMsg{
					Jid:  e.jid,
					Data: e.items[i].value,
					What: what.Value,
				})
			default:
				msgs = append(msgs, wsMsg{
					Jid:  e.jid,
					Data: e.items[i].name + "\n" + e.items[i].value,
					What: what.SAttr,
				})
			}
		}
	}
	return msgs
}

func (e *Element) ensureItemLocked(name string) *elemItem {
	if name == elemReplaceMagic && len(e.items) > 0 {
		if e.items[0].name == elemReplaceMagic {
			return &(e.items[0])
		}
		e.items = e.items[:0]
	}
	for i := range e.items {
		if e.items[i].name == name {
			return &(e.items[i])
		}
	}
	e.items = append(e.items, elemItem{name: name})
	return &(e.items[len(e.items)-1])
}

// SetAttr queues sending a new attribute value
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) SetAttr(attr, val string) (changed bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ei := e.ensureItemLocked(attr)
	if ei.remove || ei.value != val {
		ei.value = val
		ei.remove = false
		ei.dirty = true
		changed = true
	}
	return
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) RemoveAttr(attr string) (changed bool) {
	e.mu.Lock()
	ei := e.ensureItemLocked(attr)
	changed = !ei.dirty
	ei.remove = true
	ei.dirty = true
	e.mu.Unlock()
	return
}

// SetInner queues sending a new inner HTML content
// to the browser for the Element.
func (e *Element) SetInner(innerHtml template.HTML) (changed bool) {
	return e.SetAttr(elemInnerMagic, string(innerHtml))
}

// SetValue queues sending a new current input value in textual form
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) SetValue(val string) (changed bool) {
	return e.SetAttr(elemValueMagic, val)
}

// Replace replaces the elements entire HTML DOM node with new HTML code.
// If the HTML code doesn't seem to contain correct HTML ID, it panics.
func (e *Element) Replace(htmlCode template.HTML) (changed bool) {
	var b []byte
	b = append(b, "id="...)
	b = e.jid.AppendQuote(b)
	if bytes.Contains([]byte(htmlCode), b) {
		return e.SetAttr(elemReplaceMagic, string(htmlCode))
	}
	panic("jaws: Element.Replace(): expected HTML " + string(b))
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
