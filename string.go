package jaws

import (
	"html"
	"html/template"
	"sync"
)

// String wraps a mutex and a string, and implements jaws.StringSetter and jaws.HtmlGetter.
// String.JawsGetHtml() will escape the string before returning it.
type String struct {
	mu  sync.RWMutex
	val string
}

func (s *String) Set(val string) {
	s.mu.Lock()
	s.val = val
	s.mu.Unlock()
}

func (s *String) Get() (val string) {
	s.mu.RLock()
	val = s.val
	s.mu.RUnlock()
	return
}

func (s *String) String() string {
	return s.Get()
}

func (s *String) JawsGetHtml(*Element) template.HTML {
	return template.HTML(html.EscapeString(s.Get()))
}

func (s *String) JawsGetString(*Element) string {
	return s.String()
}

func (s *String) JawsSetString(e *Element, val string) error {
	s.Set(val)
	return nil
}
