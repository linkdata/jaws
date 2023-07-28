package jaws

import (
	"html/template"
	"time"

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

func MakeValueProxy(value interface{}) (vp ValueProxy) {
	switch data := value.(type) {
	case ValueProxy:
		vp = data
	case template.HTML:
		vp = &defaultValueProxy{v: data}
	case string:
		vp = &defaultValueProxy{v: data}
	case bool:
		vp = &defaultValueProxy{v: data}
	case time.Time:
		vp = &defaultValueProxy{v: data}
	case int:
		vp = &defaultValueProxy{v: float64(data)}
	case float32:
		vp = &defaultValueProxy{v: float64(data)}
	case float64:
		vp = &defaultValueProxy{v: data}
	}
	if vp == nil {
		panic("jaws: failed make a ValueProxy")
	}
	return
}
