package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	ValueProxy
}

func (ui *UiHtmlInner) WriteHtmlInner(e *Element, w io.Writer, htmltag, htmltype string, data []interface{}) {
	writeUiDebug(e, w)
	ui.UiHtml.WriteHtmlInner(w, e, htmltag, htmltype, e.ToHtml(ui.ValueProxy.JawsGet(e)), data)
}

func NewUiHtmlInner(up Params) UiHtmlInner {
	return UiHtmlInner{
		UiHtml:     NewUiHtml(up),
		ValueProxy: up.ValueProxy(),
	}
}

func (ui *UiHtmlInner) JawsUpdate(u Updater) {
	u.SetInner(u.ToHtml(ui.ValueProxy.JawsGet(u.Element)))
}
