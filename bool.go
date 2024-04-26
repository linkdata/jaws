package jaws

import (
	"strconv"
	"sync"
)

// Bool wraps a mutex and a boolean, and implements jaws.BoolSetter.
type Bool struct {
	mu    sync.Mutex
	Value bool
}

func (s *Bool) Set(val bool) {
	s.mu.Lock()
	s.Value = val
	s.mu.Unlock()
}

func (s *Bool) Get() (val bool) {
	s.mu.Lock()
	val = s.Value
	s.mu.Unlock()
	return
}

func (s *Bool) String() string {
	if s.Get() {
		return "true"
	}
	return "false"
}

func (s *Bool) JawsGetBool(*Element) bool {
	return s.Get()
}

func (s *Bool) JawsSetBool(e *Element, val bool) error {
	s.Set(val)
	return nil
}

func (s *Bool) MarshalJSON() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *Bool) UnmarshalJSON(b []byte) (err error) {
	var val bool
	if val, err = strconv.ParseBool(string(b)); err == nil {
		s.Set(val)
	}
	return
}

func (s *Bool) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *Bool) UnmarshalText(b []byte) (err error) {
	var val bool
	if val, err = strconv.ParseBool(string(b)); err == nil {
		s.Set(val)
	}
	return
}
