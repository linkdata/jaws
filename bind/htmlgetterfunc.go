package bind

import (
	"html/template"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawstags"
)

type htmlGetterFunc struct {
	fn   func(*jaws.Element) template.HTML
	tags []any
}

var _ jawstags.TagGetter = &htmlGetterFunc{}

func (g *htmlGetterFunc) JawsGetHTML(e *jaws.Element) template.HTML {
	return g.fn(e)
}

func (g *htmlGetterFunc) JawsGetTag(jawstags.Context) any {
	return g.tags
}

// HTMLGetterFunc wraps a function and returns a HTMLGetter.
func HTMLGetterFunc(fn func(elem *jaws.Element) (tmpl template.HTML), tags ...any) HTMLGetter {
	return &htmlGetterFunc{fn: fn, tags: tags}
}
