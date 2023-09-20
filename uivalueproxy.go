package jaws

type UiValueProxy struct {
	UiHtml
	ValueProxy
	ValueProxyTag interface{}
}

func (ui *UiValueProxy) parseParams(elem *Element, params []interface{}) (attrs []string) {
	if tags := TagExpand(ui.ValueProxy, nil); len(tags) > 0 {
		ui.ValueProxyTag = tags[0]
	}
	return ui.UiHtml.parseParams(elem, append(params, ui.ValueProxy))
}

func (ui *UiValueProxy) JawsUpdate(u Updater) {
	u.SetValue(htmlValueString(ui.JawsGet(u.Element)))
}

func (ui *UiValueProxy) Get(e *Element) interface{} {
	return ui.ValueProxy.JawsGet(e)
}
