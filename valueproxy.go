package jaws

import (
	"fmt"
	"html/template"
	"sync/atomic"
)

type ValueProxy interface {
	JawsGet(e *Element) (val interface{})
	JawsSet(e *Element, val interface{}) (changed bool)
}

type atomicProxy struct{ *atomic.Value }

func (vp atomicProxy) JawsGet(e *Element) interface{}           { return vp.Load() }
func (vp atomicProxy) JawsSet(e *Element, val interface{}) bool { return vp.Swap(val) != val }
func (vp atomicProxy) JawsTags(rq *Request, inTags []interface{}) (outTags []interface{}) {
	return append(inTags, vp.Value)
}

type readonlyProxy struct{ Value interface{} }

func (vp readonlyProxy) JawsGet(e *Element) interface{} { return vp.Value }
func (vp readonlyProxy) JawsSet(e *Element, val interface{}) bool {
	panic(fmt.Errorf("jaws: Element %v: ValueProxy for %T is read-only", e, val))
}

func MakeValueProxy(valtag interface{}) (vp ValueProxy) {
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
