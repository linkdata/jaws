package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	HtmlInner string
}

func (ui *UiHtmlInner) WriteHtmlInner(e *Element, w io.Writer, htmltag, htmltype string) error {
	return ui.UiHtml.WriteHtmlInner(w, htmltag, htmltype, ui.HtmlInner, e.Jid().String(), e.Data...)
}

func (ui *UiHtmlInner) JawsUpdate(e *Element) (err error) {
	if e.SetInner(ui.HtmlInner) {
		e.UpdateOthers(ui.Tags)
	}
	return nil
}
