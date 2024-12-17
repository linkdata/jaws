package jaws

type stringGetterFunc struct {
	fn   func(*Element) string
	tags []any
}

func (g *stringGetterFunc) JawsGetString(e *Element) string {
	return g.fn(e)
}

func (g *stringGetterFunc) JawsGetTag(e *Request) any {
	return g.tags
}

// StringGetterFunc wraps a function and returns a StringGetter.
func StringGetterFunc(fn func(elem *Element) (s string), tags ...any) StringGetter {
	return &stringGetterFunc{fn: fn, tags: tags}
}
