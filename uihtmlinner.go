package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	InnerProxy
}

func (ui *UiHtmlInner) WriteHtmlInner(e *Element, w io.Writer, htmltag, htmltype string, data []interface{}) error {
	return ui.UiHtml.WriteHtmlInner(w, htmltag, htmltype, string(ui.InnerProxy.JawsInner(e)), e.Jid().String(), data)
}

func NewUiHtmlInner(tags []interface{}, p InnerProxy) UiHtmlInner {
	return UiHtmlInner{
		UiHtml:     UiHtml{Tags: append(tags, p)},
		InnerProxy: p,
	}
}

func (ui *UiHtmlInner) JawsUpdate(e *Element) (err error) {
	if e.SetInner(string(ui.InnerProxy.JawsInner(e))) {
		e.UpdateOthers(ui.Tags)
	}
	return nil
}
