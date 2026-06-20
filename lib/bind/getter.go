package bind

import (
	"errors"
	"fmt"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// ErrValueNotSettable is returned by read-only adapters when [Setter.JawsSet]
// is called.
var ErrValueNotSettable = errors.New("value not settable")

// Getter exposes a value for a [jaws.Element].
type Getter[T comparable] interface {
	JawsGet(elem *jaws.Element) (value T)
}

type getterStatic[T comparable] struct {
	v T
}

func (getterStatic[T]) JawsSet(elem *jaws.Element, value T) error {
	return ErrValueNotSettable
}

func (s getterStatic[T]) JawsGet(elem *jaws.Element) T {
	return s.v
}

func (s getterStatic[T]) JawsGetTag(tag.Context) any {
	return nil
}

func makeStaticGetter[T comparable](value T) Getter[T] {
	return getterStatic[T]{value}
}

// MakeGetter returns value as a [Getter].
//
// value may be a [Getter] of the same type or a static value of type T. It panics
// for any other type. A static value becomes a read-only adapter that also
// satisfies [Setter] and returns [ErrValueNotSettable] from [Setter.JawsSet].
func MakeGetter[T comparable](value any) Getter[T] {
	switch v := value.(type) {
	case Getter[T]:
		return v
	case T:
		return makeStaticGetter(v)
	}
	var blank T
	panic(fmt.Errorf("expected bind.Getter[%T] or %T not %T", blank, blank, value))
}
