package jaws

type UiInput struct {
	UiHtml
	ValueProxy
	Tag interface{}
}

func (ui *UiInput) JawsSet(e *Element, value interface{}) (changed bool) {
	if changed = ui.ValueProxy.JawsSet(e, value); changed {
		e.Jaws.Dirty(ui.Tag)
	}
	return
}

func (ui *UiInput) parseParams(elem *Element, params []interface{}) (attrs []string) {
	if tags := TagExpand(ui.ValueProxy, nil); len(tags) > 0 {
		ui.Tag = tags[0]
	}
	return ui.UiHtml.parseParams(elem, append(params, ui.ValueProxy))
}

func (ui *UiInput) JawsUpdate(u Updater) {
	u.SetValue(htmlValueString(ui.JawsGet(u.Element)))
}

var _ ValueProxy = (*UiInput)(nil)
