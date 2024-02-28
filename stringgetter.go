package jaws

import (
	"fmt"
	"html/template"
)

type StringGetter interface {
	JawsGetString(e *Element) string
}

type stringGetter struct{ v string }

func (g stringGetter) JawsGetString(e *Element) string {
	return g.v
}

func (g stringGetter) JawsSetString(*Element, string) error {
	return ErrValueNotSettable
}

func (g stringGetter) JawsGetTag(rq *Request) any {
	return nil
}

func makeStringGetter(v any) StringGetter {
	switch v := v.(type) {
	case StringGetter:
		return v
	case string:
		return stringGetter{v}
	case template.HTML:
		return stringGetter{string(v)}
	case template.HTMLAttr:
		return stringGetter{string(v)}
	}
	panic(fmt.Errorf("expected jaws.StringGetter or string, not %T", v))
}
