package jaws

import (
	"html/template"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

type elemItem struct {
	name  string
	value *string
	dirty bool
}

const elemInnerMagic = ">I"
const elemValueMagic = ">V"

// An Element is an instance of an UI object and it's user data in a Request.
type Element struct {
	tags  []interface{}    // (read-only) tags
	jid   Jid              // (read-only) JaWS ID, unique to this Element within it's Request
	ui    UI               // (read-only) the UI object
	rq    *Request         // (read-only) the Request the Element belongs to
	Data  []interface{}    // the optional data provided to the Request.UI() call
	mu    deadlock.RWMutex // protects following
	items []elemItem       // currently known items
}

// Jid returns the JaWS ID for this element, unique within it's Request.
func (e *Element) Jid() Jid {
	return e.jid
}

// UI returns the UI object.
func (e *Element) UI() UI {
	return e.ui
}

// Request returns the Request that the Element belongs to.
func (e *Element) Request() *Request {
	return e.rq
}

// Update calls JawsUpdate for this Element's UI object.
func (e *Element) Update() error {
	return e.ui.JawsUpdate(e)
}

// Update calls JawsUpdate for all Elements except this one that have one or more of the given tags.
func (e *Element) UpdateOthers(tags []interface{}) {
	e.rq.Jaws.Broadcast(&Message{
		Tags: tags,
		What: what.Update,
		from: e,
	})
}

func (e *Element) appendTodo(msgs []wsMsg) []wsMsg {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.items {
		if e.items[i].dirty {
			e.items[i].dirty = false
			if e.items[i].value == nil {
				// delete attribute
				msgs = append(msgs, wsMsg{
					Jid:  e.jid,
					Data: e.items[i].name,
					What: what.RAttr,
				})
				continue
			}
			switch e.items[i].name {
			case elemInnerMagic:
				msgs = append(msgs, wsMsg{
					Jid:  e.jid,
					Data: *(e.items[i].value),
					What: what.Inner,
				})
			case elemValueMagic:
				msgs = append(msgs, wsMsg{
					Jid:  e.jid,
					Data: *(e.items[i].value),
					What: what.Value,
				})
			default:
				msgs = append(msgs, wsMsg{
					Jid:  e.jid,
					Data: (e.items[i].name) + "\n" + *(e.items[i].value),
					What: what.SAttr,
				})
			}
		}
	}
	return msgs
}

func (e *Element) ensureItemLocked(name string) *elemItem {
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
	ei := e.ensureItemLocked(attr)
	if ei.value == nil || *ei.value != val {
		ei.value = &val
		ei.dirty = true
		changed = true
	}
	e.mu.Unlock()
	return
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) RemoveAttr(attr string) (changed bool) {
	e.mu.Lock()
	ei := e.ensureItemLocked(attr)
	changed = !ei.dirty
	ei.value = nil
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

// Remove immediately invalidates the given JaWS ID in this Request and sends a removal request
// to the browser to remove the HTML element completely from the DOM.
func (e *Element) Remove() {
}

// Insert calls the Javascript 'insertBefore()' method on the given element.
// The position parameter 'where' may be either a JaWS ID, a child index or the text 'null'.
func (e *Element) Insert(where, html string) {
}

// Append calls the Javascript 'appendChild()' method on the given element.
func (e *Element) Append(html string) {
}

// Replace calls the Javascript 'replaceChild()' method on the given element.
// The position parameter 'where' may be either a JaWS ID or an index.
func (e *Element) Replace(where, html string) {
}
