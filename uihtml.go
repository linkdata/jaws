package jaws

type UiHtml struct {
	Tag any
}

func (ui *UiHtml) applyGetter(e *Element, getter any) {
	ui.Tag = e.ApplyGetter(getter)
}
