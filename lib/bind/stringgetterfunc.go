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

// StringGetterFunc wraps a function and returns a Getter[string]
func StringGetterFunc(fn func(elem *jaws.Element) (s string), tags ...any) Getter[string] {
	return &stringGetterFunc{fn: fn, tags: tags}
}
