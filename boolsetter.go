package jaws

import (
	"errors"
	"fmt"
	"sync/atomic"
)

type BoolSetter interface {
	JawsGetBool(e *Element) bool
	// JawsSetBool may return ErrValueUnchanged to indicate value was already set.
	JawsSetBool(e *Element, v bool) (err error)
}

type boolGetter struct{ v bool }

var ErrValueNotSettable = errors.New("value not settable")

func (g boolGetter) JawsGetBool(*Element) bool {
	return g.v
}

func (g boolGetter) JawsSetBool(*Element, bool) error {
	return ErrValueNotSettable
}

func (g boolGetter) JawsGetTag(rq *Request) any {
	return nil
}

func makeBoolSetter(v any) BoolSetter {
	switch v := v.(type) {
	case BoolSetter:
		return v
	case bool:
		return boolGetter{v}
	case *atomic.Value:
		return atomicSetter{v}
	}
	panic(fmt.Errorf("expected jaws.BoolSetter or bool, not %T", v))
}
