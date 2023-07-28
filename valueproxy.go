package jaws

import "github.com/linkdata/deadlock"

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
