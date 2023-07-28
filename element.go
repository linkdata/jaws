package jaws

// An Element is an instance of an UI object and it's user data in a Request.
type Element struct {
	jid  string        // jid
	ui   UI            // the UI object
	rq   *Request      // the Request the Element belongs to
	Data []interface{} // the optional data provided to the Request.UI() call
}

// Jid returns the 'jid' HTML attribute, unique within the Request
func (e *Element) Jid() string {
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
