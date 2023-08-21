package jaws

import (
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
	var changed bool
	dvh.mu.Lock()
	if dvh.v != val {
		dvh.v = val
		changed = true
	}
	dvh.mu.Unlock()
	if changed {
		e.Update()
	}
	return
}

func MakeValueProxy(value interface{}) (vp ValueProxy) {
	if v, ok := value.(ValueProxy); ok {
		vp = v
	} else {
		vp = &defaultValueProxy{v: value}
	}
	return
}
