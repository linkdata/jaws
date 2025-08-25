package jaws

import "sync/atomic"

type UiInput struct {
	Tag  any
	Last atomic.Value
}

func (ui *UiInput) applyGetter(e Element, getter any) (err error) {
	ui.Tag, err = e.ApplyGetter(getter)
	return
}

func (ui *UiInput) MaybeDirty(val any, e Element, err error) error {
	var changed bool
	if changed, err = e.MaybeDirty(ui.Tag, err); changed {
		ui.Last.Store(val)
	}
	return err
}
