package core

import "html/template"

type htmlGetterFunc struct {
	fn   func(*Element) template.HTML
	tags []any
}

var _ TagGetter = &htmlGetterFunc{}

func (g *htmlGetterFunc) JawsGetHTML(e *Element) template.HTML {
	return g.fn(e)
}

func (g *htmlGetterFunc) JawsGetTag(e *Request) any {
	return g.tags
}

// HTMLGetterFunc wraps a function and returns a HTMLGetter.
func HTMLGetterFunc(fn func(elem *Element) (tmpl template.HTML), tags ...any) HTMLGetter {
	return &htmlGetterFunc{fn: fn, tags: tags}
}
