package jaws

import (
	"reflect"
)

// ErrNotComparable is returned when a UI object or tag is not comparable.
var ErrNotComparable errNotComparable

type errNotComparable struct {
	t reflect.Type
}

func (e errNotComparable) Error() string {
	return e.t.String() + " is not comparable"
}

func (errNotComparable) Is(target error) bool {
	return target == ErrNotComparable
}

func newErrNotComparable(x any) error {
	if t := reflect.TypeOf(x); !t.Comparable() {
		return errNotComparable{t: t}
	}
	return nil
}
