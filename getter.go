package jaws

import (
	"html/template"
	"sync/atomic"
)

type Getter interface {
	JawsGet(e *Element) (val interface{})
}

type readonlyProxy struct{ Value interface{} }

func (vp readonlyProxy) JawsGet(e *Element) interface{} { return vp.Value }

func MakeGetter(valtag interface{}) (vp Getter) {
	switch data := valtag.(type) {
	case nil:
		// does nothing
	case *atomic.Value:
		vp = atomicProxy{Value: data}
	case *NamedBoolArray:
		vp = data
	case *NamedBool:
		vp = data
	case ValueProxy:
		vp = data
	case string:
		vp = readonlyProxy{Value: template.HTML(data)}
	case template.HTML:
		vp = readonlyProxy{Value: data}
	default:
		vp = readonlyProxy{Value: data}
	}
	return
}
