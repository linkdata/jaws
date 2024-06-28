package jaws

import (
	"sync/atomic"
)

type AnySetter interface {
	JawsGetAny(e *Element) any
	// JawsSetAny may return ErrValueUnchanged to indicate value was already set.
	JawsSetAny(e *Element, v any) (err error)
}

type anyGetter struct{ v any }

func (g anyGetter) JawsGetAny(e *Element) any {
	return g.v
}

func (g anyGetter) JawsSetAny(*Element, any) error {
	return ErrValueNotSettable
}

func (g anyGetter) JawsGetTag(rq *Request) any {
	return nil
}

func makeAnySetter(v any) AnySetter {
	switch v := v.(type) {
	case AnySetter:
		return v
	case *atomic.Value:
		return atomicSetter{v}
	}
	return anyGetter{v}
}
