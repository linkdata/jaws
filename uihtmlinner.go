package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiGetter
}

func (ui *UiHtmlInner) WriteHtmlInner(e *Element, w io.Writer, htmltag, htmltype string, params ...interface{}) {
	ui.UiHtml.WriteHtmlInner(w, e, htmltag, htmltype, e.ToHtml(ui.JawsGet(e)), ui.parseParams(e, params)...)
}

func (ui *UiHtmlInner) JawsUpdate(u Updater) {
	u.SetInner(u.ToHtml(ui.JawsGet(u.Element)))
}
