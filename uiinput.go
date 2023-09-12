package jaws

type UiInput struct {
	UiHtml
	ValueProxy
}

func NewUiInput(up Params) UiInput {
	return UiInput{
		UiHtml:     NewUiHtml(up),
		ValueProxy: up.ValueProxy(),
	}
}

func (ui *UiInput) JawsUpdate(e *Element, u Updater) {
	u.SetValue(htmlValueString(ui.JawsGet(e)))
}

func (ui *UiInput) Get(e *Element) interface{} {
	return ui.ValueProxy.JawsGet(e)
}

func (ui *UiInput) Set(e *Element, value interface{}) (changed bool) {
	if changed = ui.ValueProxy.JawsSet(e, value); changed {
		e.DirtyOthers(ui.Tags...)
	}
	return
}
