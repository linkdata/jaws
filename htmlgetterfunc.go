package jaws

import "html/template"

type htmlGetterFunc struct {
	fn   func(*Element) template.HTML
	tags []any
}

var _ TagGetter = &htmlGetterFunc{}

func (g *htmlGetterFunc) JawsGetHtml(e *Element) template.HTML {
	return g.fn(e)
}

func (g *htmlGetterFunc) JawsGetTag(e *Request) any {
	return g.tags
}

// HtmlGetterFunc wraps a function and returns a HtmlGetter.
func HtmlGetterFunc(fn func(*Element) template.HTML, tags ...any) HtmlGetter {
	return &htmlGetterFunc{fn: fn, tags: tags}
}
