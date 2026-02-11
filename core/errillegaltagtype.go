package core

import "fmt"

// ErrIllegalTagType is returned when a UI tag type is disallowed
var ErrIllegalTagType errIllegalTagType

type errIllegalTagType struct {
	tag any
}

func (e errIllegalTagType) Error() string {
	return fmt.Sprintf("illegal tag type %T", e.tag)
}

func (errIllegalTagType) Is(other error) bool {
	return other == ErrIllegalTagType
}
