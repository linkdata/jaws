package jaws

type UiInput struct {
	UiHtml
	ValueProxy
}

func NewUiInput(up Params) UiInput {
	return UiInput{
		UiHtml:     UiHtml{Tags: up.Tags()},
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

func (ui *UiInput) Set(e *Element, value interface{}) {
	if ui.ValueProxy.JawsSet(e, value) {
		e.UpdateOthers(ui.Tags)
	}
}
