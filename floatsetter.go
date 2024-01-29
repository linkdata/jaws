package jaws

import (
	"fmt"
	"sync/atomic"
)

type FloatSetter interface {
	JawsGetFloat(e *Element) float64
	JawsSetFloat(e *Element, v float64) (err error)
}

type floatGetter struct{ v float64 }

func (g floatGetter) JawsGetFloat(e *Element) float64 {
	return g.v
}

func (g floatGetter) JawsSetFloat(*Element, float64) error {
	return ErrValueNotSettable
}

func (g floatGetter) JawsGetTag(rq *Request) any {
	return nil
}

func makeFloatSetter(v any) FloatSetter {
	switch v := v.(type) {
	case FloatSetter:
		return v
	case float64:
		return floatGetter{v}
	case float32:
		return floatGetter{float64(v)}
	case int:
		return floatGetter{float64(v)}
	case *atomic.Value:
		return atomicSetter{v}
	}
	panic(fmt.Errorf("expected jaws.FloatSetter, float or int, not %T", v))
}
