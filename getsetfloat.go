package jaws

import "sync/atomic"

type FloatGetter interface {
	JawsGetFloat(rq *Request) float64
}

type FloatSetter interface {
	FloatGetter
	JawsSetFloat(rq *Request, v float64) (err error)
}

type floatGetter struct{ v float64 }

func (g floatGetter) JawsGetFloat(rq *Request) float64 {
	return g.v
}

func (g floatGetter) JawsGetTag(rq *Request) interface{} {
	return nil
}

func makeFloatGetter(v interface{}) FloatGetter {
	switch v := v.(type) {
	case FloatGetter:
		return v
	case float64:
		return floatGetter{v}
	case float32:
		return floatGetter{float64(v)}
	case int:
		return floatGetter{float64(v)}
	case *atomic.Value:
		return atomicGetter{v}
	}
	panic("makeFloatGetter: invalid type")
}
