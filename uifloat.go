package jaws

import (
	"sync"
)

var _ FloatSetter = UiFloat{}

// UiFloat implements FloatSetter given a sync.Locker (or RLocker) and a float64 pointer.
type UiFloat struct {
	L sync.Locker
	P *float64
}

func (ui UiFloat) JawsGetFloat(e *Element) (val float64) {
	if rl, ok := ui.L.(RLocker); ok {
		rl.RLock()
		val = *ui.P
		rl.RUnlock()
		return
	}
	ui.L.Lock()
	val = *ui.P
	ui.L.Unlock()
	return
}

func (ui UiFloat) JawsSetFloat(e *Element, val float64) (err error) {
	ui.L.Lock()
	if *ui.P == val {
		err = ErrValueUnchanged
	} else {
		*ui.P = val
	}
	ui.L.Unlock()
	return
}
