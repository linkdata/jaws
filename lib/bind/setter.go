package bind

import (
	"fmt"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// Setter exposes and updates a value for an [jaws.Element].
type Setter[T comparable] interface {
	Getter[T]
	// JawsSet may return [jaws.ErrValueUnchanged] to indicate value was already set.
	JawsSet(elem *jaws.Element, value T) (err error)
}

type setterReadOnly[T comparable] struct {
	Getter[T]
}

func (setterReadOnly[T]) JawsSet(elem *jaws.Element, value T) error {
	return ErrValueNotSettable
}

func (s setterReadOnly[T]) JawsGetTag(tag.Context) any {
	return s.Getter
}

type setterStatic[T comparable] struct {
	v T
}

func (setterStatic[T]) JawsSet(elem *jaws.Element, value T) error {
	return ErrValueNotSettable
}

func (s setterStatic[T]) JawsGet(elem *jaws.Element) T {
	return s.v
}

func (s setterStatic[T]) JawsGetTag(tag.Context) any {
	return nil
}

// MakeSetter returns v as a [Setter].
//
// v may be a [Setter] of the same type, a [Getter] of the same type or a
// static value of type T. Getter and static adapters are read-only and return
// [ErrValueNotSettable] from [Setter.JawsSet]. MakeSetter panics for any other
// type.
func MakeSetter[T comparable](value any) Setter[T] {
	switch v := value.(type) {
	case Setter[T]:
		return v
	case Getter[T]:
		return setterReadOnly[T]{v}
	case T:
		return setterStatic[T]{v}
	}
	var blank T
	panic(fmt.Errorf("expected jaws.Setter[%T], jaws.Getter[%T] or %T not %T", blank, blank, blank, value))
}
