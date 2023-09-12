package jaws

import (
	"fmt"
	"sync/atomic"
)

type ValueProxy interface {
	JawsGet(e *Element) (val interface{})
	JawsSet(e *Element, val interface{}) (changed bool)
}

type atomicProxy struct{ *atomic.Value }

func (vp atomicProxy) JawsGet(e *Element) interface{}           { return vp.Load() }
func (vp atomicProxy) JawsSet(e *Element, val interface{}) bool { return vp.Swap(val) != val }

type readonlyProxy struct{ Value interface{} }

func (vp readonlyProxy) JawsGet(e *Element) interface{} { return vp.Value }
func (vp readonlyProxy) JawsSet(e *Element, val interface{}) bool {
	panic(fmt.Errorf("jaws: Element %v: ValueProxy for %T is read-only", e, val))
}
