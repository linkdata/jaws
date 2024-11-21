package jaws

import (
	"fmt"
	"sync"
)

type stringizer[T any] struct {
	v *T
	f string
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

type lockedstringer struct {
	l sync.Locker
	s fmt.Stringer
}

func (s lockedstringer) String() (value string) {
	if rl, ok := s.l.(RLocker); ok {
		rl.RLock()
		defer rl.RUnlock()
	} else {
		s.l.Lock()
		defer s.l.Unlock()
	}
	return s.s.String()
}

func (s lockedstringer) JawsGetTag(*Request) any {
	return s.s
}

// Stringer returns a lock protected fmt.Stringer using fmt.Sprintf(formatting, *T).
// If formatting is omitted and *T or T implements fmt.Stringer, that will be used instead of fmt.Sprintf.
func Stringer[T any](l sync.Locker, p *T, formatting ...string) fmt.Stringer {
	var f string
	if len(formatting) > 0 {
		f = formatting[0]
	} else {
		if x, ok := any(*p).(fmt.Stringer); ok {
			return lockedstringer{l, x}
		}
		if x, ok := any(p).(fmt.Stringer); ok {
			return lockedstringer{l, x}
		}
	}
	return lockedstringer{l, stringizer[T]{p, f}}
}
