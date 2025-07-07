package jaws

// InitHandler allows initializing UI getters before their use.
//
// You can of course initialize them in the call from the template engine,
// but at that point you don't have access to the Element or the Element.Context.
type InitHandler interface {
	JawsInit(e *Element) (err error)
}
