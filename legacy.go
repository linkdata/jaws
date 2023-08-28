package jaws

import (
	"strconv"
	"time"

	"github.com/linkdata/jaws/what"
)

// Deprecated: Will be removed in future
type ClickFn = func(*Request, string) error

// Deprecated: Will be removed in future
type InputTextFn = func(*Request, string, string) error

// Deprecated: Will be removed in future
type InputBoolFn = func(*Request, string, bool) error

// Deprecated: Will be removed in future
type InputFloatFn = func(*Request, string, float64) error

// Deprecated: Will be removed in future
type InputDateFn = func(*Request, string, time.Time) error

// Deprecated: Will be removed in future
func (rq *Request) GetEventFn(tagitem interface{}) (fn EventFn, ok bool) {
	tags := ProcessTags(tagitem)
	for _, tag := range tags {
		if jid, ok := tag.(Jid); ok {
			if elem := rq.GetElement(jid); elem != nil {
				if uih, isuih := elem.UI().(*UiHtml); isuih {
					return uih.EventFn, true
				}
			}
			return nil, false
		}
	}

	rq.mu.RLock()
	defer rq.mu.RUnlock()
	var elems []*Element
	for _, tag := range tags {
		elems = append(elems, rq.tagMap[tag]...)
	}
	for _, elem := range elems {
		if uih, isuih := elem.UI().(*UiHtml); isuih {
			return uih.EventFn, true
		}
	}
	return nil, false
}

// Deprecated: Will be removed in future
func (rq *Request) SetEventFn(tagstring string, fn EventFn) {
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if elems, ok := rq.tagMap[tagstring]; ok && len(elems) > 0 {
		for _, elem := range elems {
			if uih, isuih := elem.UI().(*UiHtml); isuih {
				uih.EventFn = fn
			}
		}
	}
}

// Deprecated: Will be removed in future
func (rq *Request) RegisterEventFn(params ...interface{}) Jid {
	return rq.Register(params...)
}

// Deprecated: Will be removed in future
// OnEvent calls SetEventFn.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnEvent(tagstring string, fn EventFn) error {
	rq.SetEventFn(tagstring, fn)
	return nil
}

// Deprecated: Will be removed in future
// SetAttr queues sending a new attribute value
// to the browser for the Element with the given JaWS ID in this Request.
func (rq *Request) SetAttr(tagitem interface{}, attr, val string) {
	rq.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.SAttr,
		Data: attr + "\n" + val,
	})
}

// Deprecated: Will be removed in future
// RemoveAttr queues sending a request to remove an attribute
// to the browser for the Element with the given JaWS ID in this Request.
func (rq *Request) RemoveAttr(tagitem interface{}, attr string) {
	rq.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.RAttr,
		Data: attr,
	})
}

// Deprecated: Will be removed in future
// SetTextValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetTextValue(tagitem interface{}, val string) {
	rq.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Value,
		Data: val,
	})
}

// Deprecated: Will be removed in future
// SetFloatValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetFloatValue(tagitem interface{}, val float64) {
	rq.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Value,
		Data: strconv.FormatFloat(val, 'f', -1, 64),
	})
}

// Deprecated: Will be removed in future
// SetBoolValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetBoolValue(tagitem interface{}, val bool) {
	rq.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Value,
		Data: strconv.FormatBool(val),
	})
}

// Deprecated: Will be removed in future
// SetDateValue sends a jid and new input value to all Requests except this one.
//
// Only the requests that have registered the jid (either with Register or OnEvent) will be sent the message.
func (rq *Request) SetDateValue(tagitem interface{}, val time.Time) {
	rq.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Value,
		Data: val.Format(ISO8601),
	})
}

// Deprecated: Will be removed in future
// SetInner sends a jid and new inner HTML to all Requests.
//
// Only the requests that have registered the 'jid' (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) SetInner(tagitem interface{}, innerHtml string) {
	jw.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Inner,
		Data: innerHtml,
	})
}

// Deprecated: Will be removed in future
// SetAttr sends an HTML id and new attribute value to all Requests.
// If the value is an empty string, a value-less attribute will be added (such as "disabled")
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) SetAttr(tagitem interface{}, attr, val string) {
	jw.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.SAttr,
		Data: attr + "\n" + val,
	})
}

// Deprecated: Will be removed in future
// RemoveAttr removes a given attribute from the HTML id for all Requests.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) RemoveAttr(tagitem interface{}, attr string) {
	jw.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.RAttr,
		Data: attr,
	})
}

// Deprecated: Will be removed in future
// SetValue sends an HTML id and new input value to all Requests.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) SetValue(tagitem interface{}, val string) {
	jw.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Value,
		Data: val,
	})
}

// Deprecated: Will be removed in future
// Remove removes the HTML element(s) with the given 'jid' on all Requests.
//
// Only the requests that have registered the 'jid' (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) Remove(tagitem interface{}) {
	jw.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Remove,
	})
}

// Deprecated: Will be removed in future
// Insert calls the Javascript 'insertBefore()' method on the given element on all Requests.
// The position parameter 'where' may be either a HTML ID, an child index or the text 'null'.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) Insert(tagitem interface{}, where, html string) {
	jw.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Insert,
		Data: where + "\n" + html,
	})
}

// Deprecated: Will be removed in future
// Append calls the Javascript 'appendChild()' method on the given element on all Requests.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) Append(tagitem interface{}, html string) {
	jw.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Append,
		Data: html,
	})
}

// Deprecated: Will be removed in future
// Replace calls the Javascript 'replaceChild()' method on the given element on all Requests.
// The position parameter 'where' may be either a HTML ID or an index.
//
// Only the requests that have registered the ID (either with Register or OnEvent) will be sent the message.
func (jw *Jaws) Replace(tagitem interface{}, where, html string) {
	jw.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Replace,
		Data: where + "\n" + html,
	})
}

// Deprecated: Will be removed in future
// Trigger invokes the event handler for the given ID with a 'trigger' event on all Requests except this one.
func (rq *Request) Trigger(tagitem interface{}, val string) {
	rq.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Trigger,
		Data: val,
	})
}

// Deprecated: Will be removed in future
// Trigger invokes the event handler for the given ID with a 'trigger' event on all Requests.
func (jw *Jaws) Trigger(tagitem interface{}, val string) {
	jw.Broadcast(&Message{
		Tags: ProcessTags(tagitem),
		What: what.Trigger,
		Data: val,
	})
}
