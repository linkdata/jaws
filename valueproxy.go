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

func (dvh *defaultValueProxy) JawsGet(e *Element) (val interface{}) {
	dvh.mu.RLock()
	val = dvh.v
	dvh.mu.RUnlock()
	return
}

func (dvh *defaultValueProxy) JawsSet(e *Element, val interface{}) (err error) {
	dvh.mu.Lock()
	dvh.v = val
	dvh.mu.Unlock()
	return
}

type atomicValueProxy struct {
	*atomic.Value
}

func (vp *atomicValueProxy) JawsGet(e *Element) interface{} {
	return vp.Value.Load()
}

func (vp *atomicValueProxy) JawsSet(e *Element, val interface{}) (err error) {
	vp.Store(val)
	return nil
}

func MakeValueProxy(value interface{}) (vp ValueProxy) {
	switch v := value.(type) {
	case ValueProxy:
		vp = v
	case *atomic.Value:
		vp = &atomicValueProxy{Value: v}
	case atomic.Value:
		panic("jaws: MakeValueProxy: must pass atomic.Value by reference")
	default:
		vp = &defaultValueProxy{v: value}
	}
	return
}
