package core

type Container interface {
	// JawsContains must return a slice of hashable UI objects. The slice contents must not be modified after returning it.
	JawsContains(e *Element) (contents []UI)
}
