package jaws

type stringGetterFunc struct {
	fn func(*Element) string
}

func (g stringGetterFunc) JawsGetString(e *Element) string {
	return g.fn(e)
}

// StringGetterFunc wraps a function and returns a StringGetter.
func StringGetterFunc(fn func(*Element) string) StringGetter {
	return stringGetterFunc{fn: fn}
}
