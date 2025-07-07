package jaws

import (
	"io"
)

type UiHTMLInner struct {
	HTMLGetter
}

func (ui *UiHTMLInner) renderInner(e *Element, w io.Writer, htmltag, htmltype string, params []any) (err error) {
	if _, err = e.ApplyGetter(ui.HTMLGetter); err == nil {
		err = WriteHTMLInner(w, e.Jid(), htmltag, htmltype, ui.JawsGetHTML(e), e.ApplyParams(params)...)
	}
	return
}

func (ui *UiHTMLInner) JawsUpdate(e *Element) {
	e.SetInner(ui.JawsGetHTML(e))
}
