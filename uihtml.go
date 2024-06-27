package jaws

import (
	"io"
)

type UiHtml struct {
	Tag any
}

func (ui *UiHtml) applyGetter(e *Element, getter any) {
	ui.Tag = e.ApplyGetter(getter)
}

func (ui *UiHtml) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	if h, ok := ui.Tag.(UI); ok {
		err = h.JawsRender(e, w, params)
	}
	return
}

func (ui *UiHtml) JawsUpdate(e *Element) {
	if h, ok := ui.Tag.(UI); ok {
		h.JawsUpdate(e)
	}
}
