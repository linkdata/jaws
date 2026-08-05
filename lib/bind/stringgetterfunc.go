package bind

import (
	"slices"

	"github.com/linkdata/jaws"
)

type stringGetterFunc struct {
	fn   func(elem *jaws.Element) string
	tags []any
}

func (g *stringGetterFunc) JawsGet(elem *jaws.Element) string {
	return g.fn(elem)
}

func (g *stringGetterFunc) JawsGetTag() any {
	return g.tags
}

// StringGetterFunc wraps fn as a [Getter] for string values.
//
// Optional tags are exposed through [github.com/linkdata/jaws/lib/tag.TagGetter].
//
// The top-level slots of tags are copied, so the caller may reuse or modify the slice
// it passed. Nested containers and reference-backed tag values are not copied; keeping
// those stable remains the caller's obligation.
func StringGetterFunc(fn func(elem *jaws.Element) (s string), tags ...any) Getter[string] {
	return &stringGetterFunc{fn: fn, tags: slices.Clone(tags)}
}
