package jaws

import "fmt"

type stringizer[T any] struct {
	f string
	v *T
}

func (s stringizer[T]) String() string {
	if s.f == "" {
		return fmt.Sprint(*s.v)
	}
	return fmt.Sprintf(s.f, *s.v)
}

func (s stringizer[T]) JawsGetTag(*Request) any {
	return s.v
}

// Fmt returns a fmt.Stringer using fmt.Sprintf(formatting, *T).
// If formatting is omitted and *T or T implements fmt.Stringer, it will be returned as-is.
// If formatting is omitted fmt.Sprint(*T) is used.
func Fmt[T any](p *T, formatting ...string) fmt.Stringer {
	var f string
	if len(formatting) > 0 {
		f = formatting[0]
	} else {
		if x, ok := any(*p).(fmt.Stringer); ok {
			return x
		}
		if x, ok := any(p).(fmt.Stringer); ok {
			return x
		}
	}
	return stringizer[T]{f, p}
}
