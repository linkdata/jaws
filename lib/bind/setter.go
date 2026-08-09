package bind

import (
	"fmt"

	"github.com/linkdata/jaws"
)

// Setter exposes and updates a value for a [jaws.Element].
//
// An editable [github.com/linkdata/jaws/lib/ui.Number] or
// [github.com/linkdata/jaws/lib/ui.Range] source must expose at least one stable,
// usable source tag through [jaws.Element.ApplyGetter] for dirty-driven
// reconciliation.
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

func (s setterReadOnly[T]) JawsGetTag() any {
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

func (s setterStatic[T]) JawsGetTag() any {
	return nil
}

// MakeSetter returns value as a [Setter].
//
// value may be a [Setter] of the same type, a [Getter] of the same type or a
// static value of type T. Getter and static adapters are read-only and return
// [ErrValueNotSettable] from [Setter.JawsSet]. MakeSetter panics for any other
// type.
//
// The adapters still satisfy Setter, so [github.com/linkdata/jaws/lib/ui.Number]
// and [github.com/linkdata/jaws/lib/ui.Range] treat them as editable. Pass an
// existing Getter directly, or use [MakeGetter] for a static value, to render a
// read-only numeric control.
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
	panic(fmt.Errorf("expected bind.Setter[%T], bind.Getter[%T] or %T not %T", blank, blank, blank, value))
}
