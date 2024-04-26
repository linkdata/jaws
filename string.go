package jaws

import (
	"html"
	"html/template"
	"sync"
)

// String wraps a mutex and a string, and implements jaws.StringSetter and jaws.HtmlGetter.
// String.JawsGetHtml() will escape the string before returning it.
type String struct {
	mu    sync.Mutex
	Value string
}

func (s *String) Set(val string) {
	s.mu.Lock()
	s.Value = val
	s.mu.Unlock()
}

func (s *String) Get() (val string) {
	s.mu.Lock()
	val = s.Value
	s.mu.Unlock()
	return
}

func (s *String) String() string {
	return s.Get()
}

func (s *String) JawsGetHtml(*Element) (val template.HTML) {
	val = template.HTML(html.EscapeString(s.Get())) // #nosec G203
	return
}

func (s *String) JawsGetString(*Element) string {
	return s.String()
}

func (s *String) JawsSetString(e *Element, val string) error {
	s.Set(val)
	return nil
}

func (s *String) MarshalText() ([]byte, error) {
	return []byte(s.Get()), nil
}

func (s *String) UnmarshalText(b []byte) (err error) {
	s.Set(string(b))
	return
}
