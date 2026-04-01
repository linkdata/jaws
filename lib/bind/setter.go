package bind

import (
	"fmt"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/jtag"
)

type Setter[T comparable] interface {
	Getter[T]
	// JawsSet may return ErrValueUnchanged to indicate value was already set.
	JawsSet(elem *jaws.Element, value T) (err error)
}

type setterReadOnly[T comparable] struct {
	Getter[T]
}

func (setterReadOnly[T]) JawsSet(*jaws.Element, T) error {
	return ErrValueNotSettable
}

func (s setterReadOnly[T]) JawsGetTag(jtag.Context) any {
	return s.Getter
}

type setterStatic[T comparable] struct {
	v T
}

func (setterStatic[T]) JawsSet(*jaws.Element, T) error {
	return ErrValueNotSettable
}

func (s setterStatic[T]) JawsGet(*jaws.Element) T {
	return s.v
}

func (s setterStatic[T]) JawsGetTag(jtag.Context) any {
	return nil
}

func MakeSetter[T comparable](v any) Setter[T] {
	switch v := v.(type) {
	case Setter[T]:
		return v
	case Getter[T]:
		return setterReadOnly[T]{v}
	case T:
		return setterStatic[T]{v}
	}
	var blank T
	panic(fmt.Errorf("expected jaws.Setter[%T], jaws.Getter[%T] or %T not %T", blank, blank, blank, v))
}
