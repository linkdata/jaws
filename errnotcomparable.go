package jaws

import "fmt"

type errNotComparable struct {
	s string
}

func (e errNotComparable) Error() string {
	return fmt.Sprintf("not hashable type %s", e.s)
}

func (errNotComparable) Is(other error) bool {
	return other == ErrIllegalTagType
}

func newErrNotComparable(tag any) error {
	return errNotComparable{
		s: fmt.Sprintf("%T", tag),
	}
}

// ErrNotComparable is returned when a UI object or tag is not comparable.
var ErrNotComparable = errNotComparable{}

func checkComparable(x any) (err error) {
	defer func() {
		if recover() != nil {
			err = newErrNotComparable(x)
		}
	}()
	tmp := map[any]struct{}{}
	tmp[x] = struct{}{}
	return
}
