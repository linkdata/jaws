package jaws

type Getter[T comparable] interface {
	JawsGet(e *Element) T
}

type Setter[T comparable] interface {
	Getter[T]
	// JawsSet may return ErrValueUnchanged to indicate value was already set.
	JawsSet(e *Element, v T) (err error)
}
