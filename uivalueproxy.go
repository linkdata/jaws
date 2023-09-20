package jaws

import "github.com/linkdata/deadlock"

type UiValueProxy struct {
	UiHtml
	ValueProxy
	ValueProxyTag interface{}
}

func (ui *UiValueProxy) parseParams(elem *Element, params []interface{}) (attrs []string) {
	if tags := TagExpand(ui.ValueProxy, nil); len(tags) > 0 {
		elem.Tag(tags...)
		ui.ValueProxyTag = tags[0]
	} else if deadlock.Debug {
		panic("ValueProxy yields no tags")
	}
	return ui.UiHtml.parseParams(elem, params)
}

func (ui *UiValueProxy) JawsUpdate(u Updater) {
	u.SetValue(htmlValueString(ui.JawsGet(u.Element)))
}

func (ui *UiValueProxy) Get(e *Element) interface{} {
	return ui.ValueProxy.JawsGet(e)
}
