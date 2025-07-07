package jaws

// InitHandler allows initializing UI getters and setters before their use.
//
// You can of course initialize them in the call from the template engine,
// but at that point you don't have access to the Element, Element.Context
// or Element.Session.
type InitHandler interface {
	JawsInit(e *Element) (err error)
}
