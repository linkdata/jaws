package jaws

import (
	"fmt"
	"html/template"
	"sync/atomic"
)

type StringGetter interface {
	JawsGetString(e *Element) string
}

type StringSetter interface {
	StringGetter
	JawsSetString(e *Element, v string) (err error)
}

type stringGetter struct{ v string }

func (g stringGetter) JawsGetString(e *Element) string {
	return g.v
}

func (g stringGetter) JawsGetTag(rq *Request) interface{} {
	return nil
}

func makeStringGetter(v interface{}) StringGetter {
	switch v := v.(type) {
	case StringGetter:
		return v
	case string:
		return stringGetter{v}
	case template.HTML:
		return stringGetter{string(v)}
	case *atomic.Value:
		return atomicGetter{v}
	}
	panic(fmt.Errorf("expected jaws.StringGetter or string, not %T", v))
}
