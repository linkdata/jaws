package jaws

import (
	"fmt"
	"html/template"
	"sync/atomic"
)

type StringSetter interface {
	StringGetter
	// JawsSetString may return ErrValueUnchanged to indicate value was already set.
	JawsSetString(e *Element, v string) (err error)
}

type stringSetter struct {
	StringGetter
}

func (stringSetter) JawsSetString(e *Element, v string) (err error) {
	return ErrValueNotSettable
}

func (g stringSetter) JawsGetTag(rq *Request) any {
	return g.StringGetter
}

func makeStringSetter(v any) StringSetter {
	switch v := v.(type) {
	case StringSetter:
		return v
	case StringGetter:
		return stringSetter{v}
	case string:
		return stringGetter{v}
	case template.HTML:
		return stringGetter{string(v)}
	case template.HTMLAttr:
		return stringGetter{string(v)}
	case *atomic.Value:
		return atomicSetter{v}
	}
	panic(fmt.Errorf("expected jaws.StringSetter or string, not %T", v))
}
