package core

type stringGetterFunc struct {
	fn   func(*Element) string
	tags []any
}

func (g *stringGetterFunc) JawsGet(e *Element) string {
	return g.fn(e)
}

func (g *stringGetterFunc) JawsGetTag(e *Request) any {
	return g.tags
}

// StringGetterFunc wraps a function and returns a Getter[string]
func StringGetterFunc(fn func(elem *Element) (s string), tags ...any) Getter[string] {
	return &stringGetterFunc{fn: fn, tags: tags}
}
