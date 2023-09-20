package jaws

type UiValueProxy struct {
	UiHtml
	ValueProxy
}

func (ui *UiValueProxy) JawsUpdate(u Updater) {
	u.SetValue(htmlValueString(ui.JawsGet(u.Element)))
}

func (ui *UiValueProxy) Get(e *Element) interface{} {
	return ui.ValueProxy.JawsGet(e)
}
