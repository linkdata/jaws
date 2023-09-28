package jaws

import (
	"html/template"
	"time"

	"github.com/linkdata/jaws/what"
)

// Deprecated: Will be removed in future
type Ui interface {
	JawsUi(rq *Request, attrs ...string) template.HTML
}

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
func (rq *Request) GetEventFn(jid Jid) (fn EventFn, ok bool) {
	if elem := rq.GetElement(jid); elem != nil {
		if uih, isuih := elem.Ui().(*UiHtml); isuih {
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
			if uih, isuih := elem.Ui().(*UiHtml); isuih {
				uih.EventFn = fn
			}
		}
	}
}

// Deprecated: Will be removed in future
func (rq *Request) RegisterEventFn(tagitem interface{}, params ...interface{}) Jid {
	return rq.Register(tagitem, params...)
}

// Deprecated: Will be removed in future
// OnEvent calls SetEventFn.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnEvent(tagstring string, fn EventFn) error {
	rq.SetEventFn(tagstring, fn)
	return nil
}

// Deprecated: Will be removed in future
// Trigger invokes the event handler for the given ID with a 'trigger' event on all Requests except this one.
func (rq *Request) Trigger(tagitem interface{}, val string) {
	rq.Broadcast(Message{
		Dest: tagitem,
		What: what.Trigger,
		Data: val,
	})
}

// Deprecated: Will be removed in future
// Trigger invokes the event handler for the given ID with a 'trigger' event on all Requests.
func (jw *Jaws) Trigger(tagitem interface{}, val string) {
	jw.Broadcast(Message{
		Dest: tagitem,
		What: what.Trigger,
		Data: val,
	})
}
