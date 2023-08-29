package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	ValueProxy
}

func (ui *UiHtmlInner) WriteHtmlInner(e *Element, w io.Writer, htmltag, htmltype string, data []interface{}) error {
	writeUiDebug(e, w)
	return ui.UiHtml.WriteHtmlInner(w, e.Jid(), htmltag, htmltype, anyToHtml(ui.ValueProxy.JawsGet(e)), data)
}

func NewUiHtmlInner(up Params) UiHtmlInner {
	return UiHtmlInner{
		UiHtml:     UiHtml{Tags: up.Tags()},
		ValueProxy: up.ValueProxy(),
	}
}

func (ui *UiHtmlInner) JawsUpdate(e *Element) (err error) {
	if e.SetInner(anyToHtml(ui.ValueProxy.JawsGet(e))) {
		e.UpdateOthers(ui.Tags)
	}
	return nil
}
