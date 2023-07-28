package jaws

import (
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
func (rq *Request) GetEventFn(tagstring string) (fn EventFn, ok bool) {
	rq.mu.RLock()
	defer rq.mu.RUnlock()
	var elems []*Element
	if elems, ok = rq.tagMap[tagstring]; ok && len(elems) > 0 {
		for _, elem := range elems {
			if uih, isuih := elem.Ui.(*UiHtml); isuih {
				return uih.EventFn, true
			}
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
			if uih, isuih := elem.Ui.(*UiHtml); isuih {
				uih.EventFn = fn
			}
		}
	}
}

// Deprecated: Will be removed in future
// OnInput registers a jid and a function to be called when it's input event fires.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnInput(tagstring string, fn func(*Request, string, string) error) error {
	rq.RegisterEventFn(tagstring, func(rq *Request, evt what.What, jid, val string) (err error) {
		if evt == what.Input {
			err = fn(rq, jid, val)
		}
		return
	})
	return nil
}

// Deprecated: Will be removed in future
// OnEvent calls SetEventFn.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnEvent(tagstring string, fn EventFn) error {
	rq.SetEventFn(tagstring, fn)
	return nil
}
