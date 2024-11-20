package jaws

import "fmt"

type stringer interface {
	String() string
}

type stringizer[T any] struct {
	v *T
}

func (s stringizer[T]) String() string {
	return fmt.Sprint(*s.v)
}

func Stringer[T any](v *T) stringer {
	return stringizer[T]{v}
}
