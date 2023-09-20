package jaws

type UiInput struct {
	UiValueProxy
}

func (ui *UiInput) Set(e *Element, value interface{}) (changed bool) {
	if changed = ui.ValueProxy.JawsSet(e, value); changed {
		e.DirtyOthers(ui.ValueProxyTag)
	}
	return
}
