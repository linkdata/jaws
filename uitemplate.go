package jaws

import (
	"html/template"
)

type UiTemplate struct {
	Template
}

// NewUiTemplate returns a *UiTemplate that renders the given template using jaws.With{Dot: dot} as data.
func NewUiTemplate(templ *template.Template, dot interface{}) *UiTemplate {
	return &UiTemplate{
		Template: Template{
			Template: templ,
			Dot:      dot,
		},
	}
}

// Template renders the given template using jaws.With{Dot: dot} as data.
//
// The templ argument can either be a string, in which case Jaws.Template.Lookup() will
// be used to resolve it. Or it can be a *template.Template directly.
func (rq *Request) Template(templ interface{}, dot interface{}) template.HTML {
	var tp *template.Template
	if name, ok := templ.(string); ok {
		tp = rq.Jaws.Template.Lookup(name)
	} else {
		tp = templ.(*template.Template)
	}
	return rq.UI(NewUiTemplate(tp, dot))
}
