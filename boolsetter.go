package jaws

import (
	"errors"
	"fmt"
	"sync/atomic"
)

type BoolSetter interface {
	JawsGetBool(rq *Element) bool
	JawsSetBool(rq *Element, v bool) (err error)
}

type boolGetter struct{ v bool }

var ErrValueNotSettable = errors.New("value not settable")

func (g boolGetter) JawsGetBool(*Element) bool {
	return g.v
}

func (g boolGetter) JawsSetBool(*Element, bool) error {
	return ErrValueNotSettable
}

func (g boolGetter) JawsGetTag(rq *Request) interface{} {
	return nil
}

func makeBoolSetter(v interface{}) BoolSetter {
	switch v := v.(type) {
	case BoolSetter:
		return v
	case bool:
		return boolGetter{v}
	case *atomic.Value:
		return atomicGetter{v}
	}
	panic(fmt.Errorf("expected jaws.BoolGetter or bool, not %T", v))
}
