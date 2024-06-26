package jaws

import "sync/atomic"

type UiInput struct {
	UiHtml
	Last atomic.Value
}

func (ui *UiInput) maybeDirty(obj, val any, e *Element, err error) error {
	var changed bool
	if changed, err = e.maybeDirty(obj, err); changed {
		ui.Last.Store(val)
		e.Dirty(ui.Tag)
	}
	return err
}
