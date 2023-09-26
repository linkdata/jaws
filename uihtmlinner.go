package jaws

import (
	"io"
)

type UiHtmlInner struct {
	UiHtml
	HtmlGetter
}

func (ui *UiHtmlInner) renderInner(e *Element, w io.Writer, htmltag, htmltype string, params []interface{}) {
	if tagger, ok := ui.HtmlGetter.(TagGetter); ok {
		e.Tag(tagger.JawsGetTag(e))
	}
	writeUiDebug(e, w)
	maybePanic(WriteHtmlInner(w, e.Jid(), htmltag, htmltype, ui.JawsGetHtml(e), ui.parseParams(e, params)...))
}

func (ui *UiHtmlInner) JawsUpdate(u Updater) {
	u.SetInner(ui.JawsGetHtml(u.Element))
}
