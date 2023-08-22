package jaws

import (
	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

// An Element is an instance of an UI object and it's user data in a Request.
type Element struct {
	jid  Jid                // (read-only) JaWS ID, unique to this Element within it's Request
	ui   UI                 // (read-only) the UI object
	rq   *Request           // (read-only) the Request the Element belongs to
	mu   deadlock.RWMutex   // protects following
	data []interface{}      // the optional data provided to the Request.UI() call
	todo map[string]*string // pending actions for attributes, inner HTML or input value
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

// Update calls JawsUpdate for UI objects that have tags in common with this Element.
func (e *Element) Update() error {
	return e.ui.JawsUpdate(e)
}

// ReadData calls the given function with the data provided to the Request.UI() call locked for reading.
func (e *Element) ReadData(fn func(data []interface{})) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	fn(e.data)
}

// WriteData calls the given function with the data provided to the Request.UI() call locked for writing.
func (e *Element) WriteData(fn func(data []interface{})) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn(e.data)
}

func (e *Element) appendTodo(msgs []wsMsg) []wsMsg {
	e.mu.Lock()
	defer e.mu.Unlock()
	for k, v := range e.todo {
		delete(e.todo, k)
		if v == nil {
			// delete attribute
			msgs = append(msgs, wsMsg{
				Jid:  e.jid,
				Data: k,
				What: what.RAttr,
			})
			continue
		}
		switch k {
		case " inner":
			msgs = append(msgs, wsMsg{
				Jid:  e.jid,
				Data: *v,
				What: what.Inner,
			})
		case " value":
			msgs = append(msgs, wsMsg{
				Jid:  e.jid,
				Data: *v,
				What: what.Value,
			})
		default:
			msgs = append(msgs, wsMsg{
				Jid:  e.jid,
				Data: k + "\n" + *v,
				What: what.SAttr,
			})
		}
	}
	return msgs
}

// SetAttr queues sending a new attribute value
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) SetAttr(attr, val string) {
	e.mu.Lock()
	e.todo[attr] = &val
	e.mu.Unlock()
}

// RemoveAttr queues sending a request to remove an attribute
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) RemoveAttr(attr string) {
	e.mu.Lock()
	e.todo[attr] = nil
	e.mu.Unlock()
}

// SetInner queues sending a new inner HTML content
// to the browser for the Element.
func (e *Element) SetInner(innerHtml string) {
	e.SetAttr(" inner", innerHtml)
}

// SetValue queues sending a new current input value in textual form
// to the browser for the Element with the given JaWS ID in this Request.
func (e *Element) SetValue(val string) {
	e.SetAttr(" value", val)
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
