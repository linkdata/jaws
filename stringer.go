package jaws

import (
	"fmt"
	"sync"
)

type stringizer[T any] struct {
	v *T
}

func (s stringizer[T]) String() string {
	return fmt.Sprint(*s.v)
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

// Stringer returns a lock protected fmt.Stringer using fmt.Sprint(*T)
// unless *T or T implements fmt.Stringer, in which case that will be used.
func Stringer[T any](l sync.Locker, p *T) fmt.Stringer {
	if x, ok := any(*p).(fmt.Stringer); ok {
		return lockedstringer{l, x}
	}
	if x, ok := any(p).(fmt.Stringer); ok {
		return lockedstringer{l, x}
	}
	return lockedstringer{l, stringizer[T]{p}}
}
