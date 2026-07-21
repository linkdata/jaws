package jaws

import (
	"reflect"

	"github.com/linkdata/jaws/lib/tag"
)

// errUnusableUI reports that a value cannot be used as a [UI] value because it is a
// nil interface, not comparable at runtime, or not equal to itself as a value holding
// NaN is.
//
// It matches [tag.ErrNotUsableAsTag] and [tag.ErrNotComparable] with errors.Is, but
// carries UI-specific text: the tag package's advice to implement JawsGetTag cannot
// make a raw UI value usable as a map[UI] key, so it must not be surfaced here.
type errUnusableUI struct {
	t reflect.Type // nil when the offending value was a nil UI
}

func (e errUnusableUI) Error() (s string) {
	s = "nil"
	if e.t != nil {
		s = e.t.String()
	}
	return s + " is not usable as a jaws.UI value: it must be non-nil, comparable, and equal to itself"
}

// Is reports whether target is a tag sentinel this error stands in for, so callers
// can match it with errors.Is.
func (errUnusableUI) Is(target error) bool {
	return target == tag.ErrNotUsableAsTag || target == tag.ErrNotComparable
}

// NewErrUnusableUI returns a non-nil error when ui cannot be used as a [UI] value,
// and nil when it can.
//
// A UI value is unusable when it is a nil interface, not comparable at runtime, or not
// equal to itself as a value holding NaN is. Containers use it both as a map key and
// to render children: a non-comparable value panics when hashed and a NaN-bearing one
// never matches itself, while a nil interface is a legal map key but has no methods to
// render. A typed nil — a non-nil interface holding a nil pointer whose type
// implements [UI] — is comparable and equal to itself, so it is reported usable;
// whether its [Renderer] tolerates a nil receiver is the concrete type's
// responsibility.
//
// The returned error matches both [tag.ErrNotUsableAsTag] and [tag.ErrNotComparable]
// under errors.Is. The container widgets use it to terminate a Request handed such a
// child; a nil interface passed directly to [Request.NewElement] is instead tolerated
// as a no-op Element, so this reports it unusable only for the container's benefit.
func NewErrUnusableUI(ui UI) error {
	if ui == nil || tag.NewErrNotUsableAsTag(ui) != nil {
		return errUnusableUI{t: reflect.TypeOf(ui)}
	}
	return nil
}
