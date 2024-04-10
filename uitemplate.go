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
// The templ argument can either be a string, in which case Jaws.Template.Lookup() will
// be used to resolve it. Or it can be a *template.Template directly.
//
// Note that templates are only rendered once; adding UI tags to a template
// have no effect, since JawsUpdate is a no-op for them.
func (rq RequestWriter) Template(templ, dot any, params ...any) error {
	return rq.UI(NewUiTemplate(rq.rq.MakeTemplate(templ, dot)), params...)
}
