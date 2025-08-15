package jaws

import (
	"errors"
	"fmt"
)

var ErrValueNotSettable = errors.New("value not settable")

type Getter[T comparable] interface {
	JawsGet(elem *Element) (value T)
}

type getterStatic[T comparable] struct {
	v T
}

func (getterStatic[T]) JawsSet(*Element, T) error {
	return ErrValueNotSettable
}

func (s getterStatic[T]) JawsGet(*Element) T {
	return s.v
}

func (s getterStatic[T]) JawsGetTag(*Request) any {
	return nil
}

func makeStaticGetter[T comparable](v T) Getter[T] {
	return getterStatic[T]{v}
}

func makeGetter[T comparable](v any) Getter[T] {
	switch v := v.(type) {
	case Getter[T]:
		return v
	case T:
		return makeStaticGetter(v)
	}
	var blank T
	panic(fmt.Errorf("expected jaws.Getter[%T] or %T not %T", blank, blank, v))
}
