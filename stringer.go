package jaws

import (
	"fmt"
	"sync"
)

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
// If formatting is omitted fmt.Sprint(*T) is used.
// If formatting is omitted and *T or T implements fmt.Stringer, that will be used instead of fmt.Sprint.
func Stringer[T any](l sync.Locker, p *T, formatting ...string) fmt.Stringer {
	return lockedstringer{l, Fmt(p, formatting...)}
}
