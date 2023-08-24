package jaws

import (
	"sync/atomic"
)

type ValueProxy interface {
	JawsGet(e *Element) (val interface{})
	JawsSet(e *Element, val interface{}) (changed bool)
}

type AtomicValueProxy struct{ *atomic.Value }

func (vp AtomicValueProxy) JawsGet(e *Element) interface{} {
	return vp.Load()
}

func (vp AtomicValueProxy) JawsSet(e *Element, val interface{}) (changed bool) {
	changed = vp.Swap(val) != val
	return
}

func MakeValueProxy(value interface{}) (vp ValueProxy) {
	switch v := value.(type) {
	case ValueProxy:
		vp = v
	case *atomic.Value:
		vp = AtomicValueProxy{Value: v}
	case atomic.Value:
		panic("jaws: MakeValueProxy: must pass atomic.Value by reference")
	default:
		panic("jaws: MakeValueProxy: expected ValueProxy or *atomic.Value")
	}
	return
}
