package jaws

import (
	"io"
)

type UiHTMLInner struct {
	HTMLGetter
}

func (ui *UiHTMLInner) renderInner(e *Element, w io.Writer, htmltag, htmltype string, params []any) error {
	e.ApplyGetter(ui.HTMLGetter)
	return WriteHTMLInner(w, e.Jid(), htmltag, htmltype, ui.JawsGetHTML(e), e.ApplyParams(params)...)
}

func (ui *UiHTMLInner) JawsUpdate(e *Element) {
	e.SetInner(ui.JawsGetHTML(e))
}
