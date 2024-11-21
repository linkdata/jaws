package jaws

import "fmt"

type stringerGetter struct{ v fmt.Stringer }

func (g stringerGetter) JawsGetString(e *Element) string {
	return g.v.String()
}

func (g stringerGetter) JawsSetString(*Element, string) error {
	return ErrValueNotSettable
}

func (g stringerGetter) JawsGetTag(rq *Request) any {
	return g.v
}
