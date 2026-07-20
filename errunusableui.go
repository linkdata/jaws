package jaws

import (
	"reflect"

	"github.com/linkdata/jaws/lib/tag"
)

// errUnusableUI reports that a value cannot be used as a [UI] value because it is
// not comparable at runtime, or not equal to itself as a value holding NaN is.
//
// It matches [tag.ErrNotUsableAsTag] and [tag.ErrNotComparable] with errors.Is, but
// carries UI-specific text: the tag package's advice to implement JawsGetTag cannot
// make a raw UI value usable as a map[UI] key, so it must not be surfaced here.
type errUnusableUI struct {
	t reflect.Type
}

func (e errUnusableUI) Error() (s string) {
	s = "value"
	if e.t != nil {
		s = e.t.String()
	}
	return s + " is not usable as a jaws.UI value: it must be comparable and equal to itself"
}

// Is reports whether target is a tag sentinel this error stands in for, so callers
// can match it with errors.Is.
func (errUnusableUI) Is(target error) bool {
	return target == tag.ErrNotUsableAsTag || target == tag.ErrNotComparable
}

// NewErrUnusableUI returns a non-nil error when ui cannot be used as a [UI] value,
// and nil when it can.
//
// A UI value is unusable when it is not comparable at runtime, or not equal to itself
// as a value holding NaN is, because it is used as a map key and would either panic
// or fail to match. The returned error matches [tag.ErrNotUsableAsTag] with errors.Is.
// [Request.NewElement] and the container widgets use it to terminate a Request handed
// such a value.
func NewErrUnusableUI(ui UI) error {
	if tag.NewErrNotUsableAsTag(ui) != nil {
		return errUnusableUI{t: reflect.TypeOf(ui)}
	}
	return nil
}
