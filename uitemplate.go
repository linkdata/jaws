package jaws

import (
	"html/template"
)

type UiTemplate struct {
	Template
}

// NewUiTemplate returns a UiTemplate that renders the given template using jaws.With{Dot: dot} as data.
func NewUiTemplate(t Template) UiTemplate {
	return UiTemplate{Template: t}
}

// Template renders the given template using jaws.With{Dot: dot} as data.
//
// The templ argument can either be a string, in which case Jaws.Template.Lookup() will
// be used to resolve it. Or it can be a *template.Template directly.
func (rq *Request) Template(templ, dot interface{}) template.HTML {
	return rq.UI(NewUiTemplate(rq.MakeTemplate(templ, dot)))
}
