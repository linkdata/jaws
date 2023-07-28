package jaws

// An Element is an instance of an UI object and it's user data in a Request.
type Element struct {
	jid      string
	Ui       UI            // the UI object
	Data     []interface{} // the optional data provided to the Request.UI() call
	*Request               // (read-only) the Request the Element belongs to
}

// Jid returns the 'jid' HTML attribute, unique within the Request
func (e *Element) Jid() string {
	return e.jid
}
