package jaws

import "html/template"

type htmlGetterFunc struct {
	fn   func(ElementIf) template.HTML
	tags []any
}

var _ TagGetter = &htmlGetterFunc{}

func (g *htmlGetterFunc) JawsGetHTML(e ElementIf) template.HTML {
	return g.fn(e)
}

func (g *htmlGetterFunc) JawsGetTag(rq RequestIf) any {
	return g.tags
}

// HTMLGetterFunc wraps a function and returns a HTMLGetter.
func HTMLGetterFunc(fn func(elem ElementIf) (tmpl template.HTML), tags ...any) HTMLGetter {
	return &htmlGetterFunc{fn: fn, tags: tags}
}
