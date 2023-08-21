package jaws

import "strconv"

// An Element is an instance of an UI object and it's user data in a Request.
type Element struct {
	jid  int           // jid
	ui   UI            // the UI object
	rq   *Request      // the Request the Element belongs to
	Data []interface{} // the optional data provided to the Request.UI() call
}

func jidToString(jid int) string {
	if jid <= 0 {
		for k, v := range metaIds {
			if v == jid {
				return k.(string)
			}
		}
		return ""
	}
	return strconv.Itoa(jid)
}

// Jid returns the 'jid' HTML attribute, unique within the Request
func (e *Element) Jid() string {
	return jidToString(e.jid)
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
func (e *Element) Update() {
	e.rq.Update(e.ui.JawsTags(e.rq))
}
