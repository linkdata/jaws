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

func (ui *UiInput) JawsUpdate(e *Element) (err error) {
	e.SetValue(htmlValueString(ui.JawsGet(e)))
	return nil
}

func (ui *UiInput) Get(e *Element) interface{} {
	return ui.ValueProxy.JawsGet(e)
}

func (ui *UiInput) Set(e *Element, value interface{}) (changed bool) {
	if changed = ui.ValueProxy.JawsSet(e, value); changed {
		e.UpdateOthers(ui.Tags...)
	}
	return
}
