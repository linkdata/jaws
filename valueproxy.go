package jaws

import (
	"fmt"
	"sync/atomic"
)

type ValueProxy interface {
	Getter
	JawsSet(e *Element, val interface{}) (changed bool)
}

type atomicProxy struct{ *atomic.Value }

func (vp atomicProxy) JawsGet(e *Element) interface{}           { return vp.Load() }
func (vp atomicProxy) JawsSet(e *Element, val interface{}) bool { return vp.Swap(val) != val }

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
	default:
		panic(fmt.Errorf("jaws.MakeValueProxy(%v): impossible", valtag))
	}
	return
}
