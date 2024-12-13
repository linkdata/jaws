package jaws

import "fmt"

type Setter[T comparable] interface {
	Getter[T]
	// JawsSet may return ErrValueUnchanged to indicate value was already set.
	JawsSet(elem *Element, value T) (err error)
}

type setterReadOnly[T comparable] struct {
	Getter[T]
}

func (setterReadOnly[T]) JawsSet(*Element, T) error {
	return ErrValueNotSettable
}

func (s setterReadOnly[T]) JawsGetTag(*Request) any {
	return s.Getter
}

func makeReadOnlySetter[T comparable](g Getter[T]) Setter[T] {
	return setterReadOnly[T]{g}
}

type setterStatic[T comparable] struct {
	v T
}

func (setterStatic[T]) JawsSet(*Element, T) error {
	return ErrValueNotSettable
}

func (s setterStatic[T]) JawsGet(*Element) T {
	return s.v
}

func (s setterStatic[T]) JawsGetTag(*Request) any {
	return nil
}

func makeStaticSetter[T comparable](v T) Setter[T] {
	return setterStatic[T]{v}
}

func makeSetter[T comparable](v any) Setter[T] {
	switch v := v.(type) {
	case Setter[T]:
		return v
	case Getter[T]:
		return makeReadOnlySetter(v)
	case T:
		return makeStaticSetter(v)
	}
	var blank T
	panic(fmt.Errorf("jaws.Setter[%T], jaws.Getter[%T] or %T not %T", blank, blank, blank, v))
}
