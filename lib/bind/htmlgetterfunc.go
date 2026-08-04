package bind

import (
	"html/template"
	"slices"

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

func (g *htmlGetterFunc) JawsGetTag() any {
	return g.tags
}

// HTMLGetterFunc wraps fn as an [HTMLGetter].
//
// Optional tags are exposed through [tag.TagGetter].
//
// The top-level slots of tags are copied, so the caller may reuse or modify the slice
// it passed. Nested containers and reference-backed tag values are not copied; keeping
// those stable remains the caller's obligation.
func HTMLGetterFunc(fn func(elem *jaws.Element) (tmpl template.HTML), tags ...any) HTMLGetter {
	return &htmlGetterFunc{fn: fn, tags: slices.Clone(tags)}
}
