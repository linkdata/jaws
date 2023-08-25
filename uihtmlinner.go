package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	ValueReader
}

func (ui *UiHtmlInner) WriteHtmlInner(e *Element, w io.Writer, htmltag, htmltype string, data []interface{}) error {
	writeUiDebug(e, w)
	return ui.UiHtml.WriteHtmlInner(w, htmltag, htmltype, anyToHtml(ui.ValueReader.JawsGet(e)), e.Jid().String(), data)
}

func NewUiHtmlInner(up Params) UiHtmlInner {
	return UiHtmlInner{
		UiHtml:      UiHtml{Tags: up.Tags()},
		ValueReader: up.ValueReader(),
	}
}

func (ui *UiHtmlInner) JawsUpdate(e *Element) (err error) {
	if e.SetInner(anyToHtml(ui.ValueReader.JawsGet(e))) {
		e.UpdateOthers(ui.Tags)
	}
	return nil
}
