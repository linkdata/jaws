package bind

import (
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

type stringGetterFunc struct {
	fn   func(*jaws.Element) string
	tags []any
}

func (g *stringGetterFunc) JawsGet(e *jaws.Element) string {
	return g.fn(e)
}

func (g *stringGetterFunc) JawsGetTag(tag.Context) any {
	return g.tags
}

// StringGetterFunc wraps fn as a [Getter] for string values.
//
// Optional tags are exposed through [tag.TagGetter].
func StringGetterFunc(fn func(elem *jaws.Element) (s string), tags ...any) Getter[string] {
	return &stringGetterFunc{fn: fn, tags: tags}
}
