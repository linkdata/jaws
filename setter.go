package jaws

type Getter[T comparable] interface {
	JawsGet(elem *Element) (value T)
}

type Setter[T comparable] interface {
	Getter[T]
	// JawsSet may return ErrValueUnchanged to indicate value was already set.
	JawsSet(elem *Element, value T) (err error)
}
