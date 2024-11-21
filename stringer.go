package jaws

import "fmt"

type stringizer[T any] struct {
	v *T
}

func (s stringizer[T]) String() string {
	return fmt.Sprint(*s.v)
}

func (s stringizer[T]) JawsGetTag(*Request) any {
	return s.v
}

func Stringer[T any](v *T) fmt.Stringer {
	if x, ok := any(*v).(fmt.Stringer); ok {
		return x
	}
	if x, ok := any(v).(fmt.Stringer); ok {
		return x
	}
	return stringizer[T]{v}
}
