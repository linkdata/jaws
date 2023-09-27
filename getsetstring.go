package jaws

import (
	"html/template"
	"sync/atomic"
)

type StringGetter interface {
	JawsGetString(rq *Request) string
}

type StringSetter interface {
	StringGetter
	JawsSetString(rq *Request, v string) (err error)
}

type stringGetter struct{ v string }

func (g stringGetter) JawsGetString(rq *Request) string {
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
	panic("makeStringGetter: invalid type")
}
