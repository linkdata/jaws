package jaws

import "sync/atomic"

type BoolGetter interface {
	JawsGetBool(e *Element) bool
}

type BoolSetter interface {
	BoolGetter
	JawsSetBool(e *Element, v bool) (err error)
}

type boolGetter struct{ v bool }

func (g boolGetter) JawsGetBool(e *Element) bool {
	return g.v
}

func (g boolGetter) JawsGetTag(rq *Request) interface{} {
	return nil
}

func makeBoolGetter(v interface{}) BoolGetter {
	switch v := v.(type) {
	case BoolGetter:
		return v
	case bool:
		return boolGetter{v}
	case *atomic.Value:
		return atomicGetter{v}
	}
	panic("makeBoolGetter: invalid type")
}
