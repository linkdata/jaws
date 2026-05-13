package bind

import (
	"html/template"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

type htmlGetterFunc struct {
	fn   func(elem *jaws.Element) template.HTML
	tags []any
}

var _ tag.TagGetter = &htmlGetterFunc{}

func (g *htmlGetterFunc) JawsGetHTML(elem *jaws.Element) template.HTML {
	return g.fn(elem)
}

func (g *htmlGetterFunc) JawsGetTag(tag.Context) any {
	return g.tags
}

// HTMLGetterFunc wraps fn as an [HTMLGetter].
//
// Optional tags are exposed through [tag.TagGetter].
func HTMLGetterFunc(fn func(elem *jaws.Element) (tmpl template.HTML), tags ...any) HTMLGetter {
	return &htmlGetterFunc{fn: fn, tags: tags}
}
