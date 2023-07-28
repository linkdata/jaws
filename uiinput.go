package jaws

type UiInput struct {
	UiHtml
	ValueProxy
}

func (ui *UiInput) Get(e *Element) interface{} {
	return ui.ValueProxy.JawsGet(e)
}

func (ui *UiInput) Set(e *Element, value interface{}) (err error) {
	return ui.ValueProxy.JawsSet(e, value)
}
