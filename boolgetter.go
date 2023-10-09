package jaws

import (
	"fmt"
	"sync/atomic"
)

type BoolGetter interface {
	JawsGetBool(rq *Element) bool
}

type BoolSetter interface {
	BoolGetter
	JawsSetBool(rq *Element, v bool) (err error)
}

type boolGetter struct{ v bool }

func (g boolGetter) JawsGetBool(rq *Element) bool {
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
	panic(fmt.Errorf("expected jaws.BoolGetter or bool, not %T", v))
}
