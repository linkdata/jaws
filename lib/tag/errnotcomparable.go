package tag

import "reflect"

// ErrNotComparable is returned when a UI object or tag is not comparable.
var ErrNotComparable errNotComparable

type errNotComparable struct {
	t reflect.Type
}

func (e errNotComparable) Error() (s string) {
	if e.t != nil {
		s = e.t.String() + " is "
	}
	return s + "not comparable"
}

func (errNotComparable) Is(target error) bool {
	return target == ErrNotComparable
}

// NewErrNotComparable returns ErrNotComparable if x is not comparable.
func NewErrNotComparable(x any) error {
	if x != nil {
		if v := reflect.ValueOf(x); !v.Comparable() {
			return errNotComparable{t: reflect.TypeOf(x)}
		}
	}
	return nil
}
