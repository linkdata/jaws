package tag

import "fmt"

// ErrIllegalTagType is returned when a UI tag type is disallowed.
var ErrIllegalTagType errIllegalTagType

type errIllegalTagType struct {
	tag any
}

func (e errIllegalTagType) Error() string {
	if e.tag == nil {
		return "illegal tag type"
	}
	return fmt.Sprintf("illegal tag type %T", e.tag)
}

func (errIllegalTagType) Is(target error) bool {
	return target == ErrIllegalTagType
}
