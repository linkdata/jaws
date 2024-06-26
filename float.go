package jaws

import (
	"strconv"
	"sync"
)

var _ FloatSetter = &Float{}

// Float wraps a mutex and a float64, and implements jaws.FloatSetter.
type Float struct {
	mu    sync.Mutex
	Value float64
}

func (s *Float) Set(val float64) {
	s.mu.Lock()
	s.Value = val
	s.mu.Unlock()
}

func (s *Float) Get() (val float64) {
	s.mu.Lock()
	val = s.Value
	s.mu.Unlock()
	return
}

func (s *Float) Swap(val float64) (old float64) {
	s.mu.Lock()
	old, s.Value = s.Value, val
	s.mu.Unlock()
	return
}

func (s *Float) String() string {
	return strconv.FormatFloat(s.Get(), 'f', -1, 64)
}

func (s *Float) JawsGetFloat(*Element) float64 {
	return s.Get()
}

func (s *Float) JawsSetFloat(e *Element, val float64) error {
	if s.Swap(val) == val {
		return ErrValueUnchanged
	}
	return nil
}

func (s *Float) MarshalJSON() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *Float) UnmarshalJSON(b []byte) (err error) {
	var val float64
	if val, err = strconv.ParseFloat(string(b), 64); err == nil {
		s.Set(val)
	}
	return
}
