package jaws

import "fmt"

type errNotHashableType struct {
	s string
}

func (e errNotHashableType) Error() string {
	return fmt.Sprintf("not hashable type %s", e.s)
}

func (errNotHashableType) Is(other error) bool {
	return other == ErrIllegalTagType
}

func newErrNotHashableType(tag any) error {
	return errNotHashableType{
		s: fmt.Sprintf("%T", tag),
	}
}

var ErrNotHashableType = errNotHashableType{}

func checkHashable(tag any) (err error) {
	defer func() {
		if recover() != nil {
			err = newErrNotHashableType(tag)
		}
	}()
	tmp := map[any]struct{}{}
	tmp[tag] = struct{}{}
	return
}
