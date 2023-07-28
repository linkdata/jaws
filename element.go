package jaws

// An Element is an instance of an UI object and it's user data in a Request.
// Consider it read-only; modifying it may lead to undefined behavior.
type Element struct {
	Ui       UI            // the UI object
	Jid      string        // the 'jid' HTML attribute, unique within the Request
	Data     []interface{} // the optional data provided to the Request.UI() call
	*Request               // the Request the Element belongs to
}
