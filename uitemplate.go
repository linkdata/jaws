package jaws

type UiTemplate struct {
	Template
}

// NewUiTemplate returns a UiTemplate that renders the given jaws.Template using jaws.With{Dot: dot} as data.
func NewUiTemplate(t Template) UiTemplate {
	return UiTemplate{Template: t}
}

// Template renders the given template using jaws.With{Dot: dot} as data.
//
// The name argument is a string to be resolved to a *template.Template
// using Jaws.LookupTemplate().
func (rq RequestWriter) Template(name string, dot any, params ...any) error {
	return rq.UI(NewUiTemplate(Template{Name: name, Dot: dot}), params...)
}
