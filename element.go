package jaws

// An Element is an instance of an UI object and it's user data in a Request.
// Consider it read-only; modifying it may lead to undefined behavior.
type Element struct {
	Jid  string
	Ui   UI
	Data []interface{}
}
