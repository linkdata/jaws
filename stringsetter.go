package jaws

import (
	"fmt"
	"html/template"
	"sync/atomic"
)

type StringSetter interface {
	JawsGetString(e *Element) string
	JawsSetString(e *Element, v string) (err error)
}

type stringGetter struct{ v string }

func (g stringGetter) JawsGetString(e *Element) string {
	return g.v
}

func (g stringGetter) JawsSetString(*Element, string) error {
	return ErrValueNotSettable
}

func (g stringGetter) JawsGetTag(rq *Request) interface{} {
	return nil
}

func makeStringSetter(v interface{}) StringSetter {
	switch v := v.(type) {
	case StringSetter:
		return v
	case string:
		return stringGetter{v}
	case template.HTML:
		return stringGetter{string(v)}
	case *atomic.Value:
		return atomicGetter{v}
	}
	panic(fmt.Errorf("expected jaws.StringSetter or string, not %T", v))
}
