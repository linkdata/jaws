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

type stringSetterT struct {
	Setter[string]
}

func (g stringSetterT) JawsGetString(e *Element) string {
	return g.JawsGet(e)
}

func (g stringSetterT) JawsSetString(e *Element, v string) (err error) {
	return g.JawsSet(e, v)
}

func (g stringSetterT) JawsGetTag(rq *Request) any {
	return g.Setter
}

func makeStringSetter(v any) StringSetter {
	switch v := v.(type) {
	case StringSetter:
		return v
	case StringGetter:
		return stringSetter{v}
	case Setter[string]:
		return stringSetterT{v}
	case Getter[string]:
		return stringGetterT{v}
	case Stringer:
		return stringerGetter{v}
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
