package jaws

import "sync/atomic"

type BoolGetter interface {
	JawsGetBool(rq *Request) bool
}

type BoolSetter interface {
	BoolGetter
	JawsSetBool(rq *Request, v bool) (err error)
}

type boolGetter struct{ v bool }

func (g boolGetter) JawsGetBool(rq *Request) bool {
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
