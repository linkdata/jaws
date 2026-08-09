package bind

import (
	"errors"
	"fmt"

	"github.com/linkdata/jaws"
)

// ErrValueNotSettable is returned by read-only adapters when [Setter.JawsSet]
// is called.
var ErrValueNotSettable = errors.New("value not settable")

// Getter exposes a value for a [jaws.Element].
//
// A changing Getter needs a stable, usable source tag, either through its own
// identity or [github.com/linkdata/jaws/lib/tag.TagGetter], for dirty-driven updates
// to reach UI elements that use it. A static Getter does not need a tag.
type Getter[T comparable] interface {
	JawsGet(elem *jaws.Element) (value T)
}

type getterStatic[T comparable] struct {
	v T
}

func (s getterStatic[T]) JawsGet(elem *jaws.Element) T {
	return s.v
}

func (s getterStatic[T]) JawsGetTag() any {
	return nil
}

func makeStaticGetter[T comparable](value T) Getter[T] {
	return getterStatic[T]{value}
}

// MakeGetter returns value as a [Getter].
//
// value may be a [Getter] of the same type or a static value of type T. It panics
// for any other type. An existing Getter is returned unchanged. A static value
// becomes an untagged Getter that does not satisfy [Setter].
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
