package jaws

import (
	"html/template"

	"github.com/linkdata/jaws/core/tags"
)

type htmlGetterFunc struct {
	fn   func(*Element) template.HTML
	tags []any
}

var _ tags.TagGetter = &htmlGetterFunc{}

func (g *htmlGetterFunc) JawsGetHTML(e *Element) template.HTML {
	return g.fn(e)
}

func (g *htmlGetterFunc) JawsGetTag(tags.Context) any {
	return g.tags
}

// HTMLGetterFunc wraps a function and returns a HTMLGetter.
func HTMLGetterFunc(fn func(elem *Element) (tmpl template.HTML), tags ...any) HTMLGetter {
	return &htmlGetterFunc{fn: fn, tags: tags}
}
