package jaws

type Container interface {
	// JawsContains must return a slice of UI objects. The slice contents must not be modified after returning it.
	JawsContains(e *Element) (contents []UI)
}
