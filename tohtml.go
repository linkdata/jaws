package jaws

import (
	"fmt"
	"html"
	"html/template"
)

type formathtml struct {
	s fmt.Stringer
	f string
}

func (fh formathtml) JawsGetHtml(*Element) template.HTML {
	s := html.EscapeString(fh.s.String())
	if fh.f != "" {
		s = fmt.Sprintf(fh.f, s)
	}
	return template.HTML(s)
}

func (fh formathtml) JawsGetTag(*Request) any {
	return fh.s
}

// ToHTML return a jaws.HtmlGetter using the given stringer.
// The string returned from the stringer will be escaped using html.EscapeString.
// If formatting is provided the escaped result is passed to fmt.Sprintf(formatting, escapedstring)
// Make sure any provided formatting produces correct HTML.
func ToHTML(stringer fmt.Stringer, formatting ...string) HtmlGetter {
	var f string
	if len(formatting) > 0 {
		f = formatting[0]
	}
	return formathtml{stringer, f}
}
