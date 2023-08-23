package jaws

import (
	"sync/atomic"

	"github.com/linkdata/deadlock"
)

type ValueProxy interface {
	JawsGet(e *Element) (val interface{})
	JawsSet(e *Element, val interface{}) (err error)
}

type defaultValueProxy struct {
	mu deadlock.RWMutex
	v  interface{}
}

func (vp *defaultValueProxy) JawsGet(e *Element) (val interface{}) {
	vp.mu.RLock()
	val = vp.v
	vp.mu.RUnlock()
	return
}

func (vp *defaultValueProxy) JawsSet(e *Element, val interface{}) (err error) {
	vp.mu.Lock()
	changed := vp.v != val
	vp.v = val
	vp.mu.Unlock()
	if changed {
		e.UpdateOthers([]interface{}{vp})
	}
	return
}

type atomicValueProxy struct{ *atomic.Value }

func (vp atomicValueProxy) JawsGet(e *Element) interface{} {
	return vp.Load()
}

func (vp atomicValueProxy) JawsSet(e *Element, val interface{}) (err error) {
	if vp.Swap(val) != val {
		e.UpdateOthers([]interface{}{vp})
	}
	return nil
}

func MakeValueProxy(value interface{}) (vp ValueProxy) {
	switch v := value.(type) {
	case ValueProxy:
		vp = v
	case *atomic.Value:
		vp = atomicValueProxy{Value: v}
	case atomic.Value:
		panic("jaws: MakeValueProxy: must pass atomic.Value by reference")
	default:
		vp = &defaultValueProxy{v: value}
	}
	return
}
