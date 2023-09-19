package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	ValueProxy
}

func (ui *UiHtmlInner) WriteHtmlInner(e *Element, w io.Writer, htmltag, htmltype string, params ...interface{}) {
	ui.ExtractParams(e.Request, ui.ValueProxy, params)
	ui.UiHtml.WriteHtmlInner(w, e, htmltag, htmltype, e.ToHtml(ui.ValueProxy.JawsGet(e)), params...)
}

func NewUiHtmlInner(vp ValueProxy) UiHtmlInner {
	return UiHtmlInner{
		ValueProxy: vp,
	}
}

func (ui *UiHtmlInner) JawsUpdate(u Updater) {
	u.SetInner(u.ToHtml(ui.ValueProxy.JawsGet(u.Element)))
}
