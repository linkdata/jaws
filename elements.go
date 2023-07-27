package jaws

import (
	"github.com/linkdata/jaws/what"
)

// OnTrigger registers a jid and a function to be called when Trigger is called for it.
// Returns a nil error so it can be used inside templates.
func (rq *Request) OnTrigger(jid string, fn ClickFn) error {
	rq.maybeEvent(what.Trigger, jid, fn)
	return nil
}
