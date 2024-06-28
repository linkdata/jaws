package jaws

import "sync"

var _ BoolSetter = UiBool{}

// UiBool implements BoolSetter given a sync.Locker (or RLocker) and a bool pointer.
type UiBool struct {
	L sync.Locker
	P *bool
}

func (ui UiBool) JawsGetBool(e *Element) (val bool) {
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

func (ui UiBool) JawsSetBool(e *Element, val bool) (err error) {
	ui.L.Lock()
	if *ui.P == val {
		err = ErrValueUnchanged
	} else {
		*ui.P = val
	}
	ui.L.Unlock()
	return
}
