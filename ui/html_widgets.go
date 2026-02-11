package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

// HTMLInner is a reusable base for widgets that render as `<tag>inner</tag>`.
type HTMLInner struct {
	HTMLGetter pkg.HTMLGetter
}

func (ui *HTMLInner) renderInner(e *pkg.Element, w io.Writer, htmlTag, htmlType string, params []any) (err error) {
	if _, err = e.ApplyGetter(ui.HTMLGetter); err == nil {
		err = pkg.WriteHTMLInner(w, e.Jid(), htmlTag, htmlType, ui.HTMLGetter.JawsGetHTML(e), e.ApplyParams(params)...)
	}
	return
}

func (ui *HTMLInner) JawsUpdate(e *pkg.Element) {
	e.SetInner(ui.HTMLGetter.JawsGetHTML(e))
}
