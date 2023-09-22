package jaws

type UiGetter struct {
	UiHtml
	Getter
	Tag interface{}
}

func (ui *UiGetter) parseParams(elem *Element, params []interface{}) (attrs []string) {
	if tags := TagExpand(ui.Getter, nil); len(tags) > 0 {
		ui.Tag = tags[0]
	}
	return ui.UiHtml.parseParams(elem, append(params, ui.Getter))
}

func (ui *UiGetter) JawsUpdate(u Updater) {
	u.SetValue(htmlValueString(ui.JawsGet(u.Element)))
}

var _ Getter = UiGetter{}
