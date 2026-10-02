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

// NewErrNotComparable returns [ErrNotComparable] if x is not comparable.
func NewErrNotComparable(x any) (err error) {
	defer func() {
		if recover() != nil {
			err = errNotComparable{t: reflect.TypeOf(x)}
		}
	}()
	// A map lookup checks every part of x, including zero-length arrays and
	// fields after a NaN, without storing x or changing its equality semantics.
	var keys map[any]struct{}
	_ = keys[x]
	return nil
}
