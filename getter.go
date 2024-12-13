package jaws

type Getter[T comparable] interface {
	JawsGet(elem *Element) (value T)
}
