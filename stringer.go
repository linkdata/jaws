package jaws

import "fmt"

type stringer interface {
	String() string
}

type stringizer struct {
	v *any
}

func (s stringizer) String() string {
	if s.v == nil {
		return "<nil>"
	}
	return fmt.Sprint(*s.v)
}

func Stringer(v *any) stringer {
	return stringizer{v}
}
