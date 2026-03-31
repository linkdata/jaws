package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
	"github.com/linkdata/jaws/core/htmlx"
)

// HTMLInner is a reusable base for widgets that render as `<tag>inner</tag>`.
type HTMLInner struct {
	HTMLGetter bind.HTMLGetter
}

func (ui *HTMLInner) renderInner(e *core.Element, w io.Writer, htmlTag, htmlType string, params []any) (err error) {
	if _, err = e.ApplyGetter(ui.HTMLGetter); err == nil {
		err = htmlx.WriteHTMLInner(w, e.Jid(), htmlTag, htmlType, ui.HTMLGetter.JawsGetHTML(e), e.ApplyParams(params)...)
	}
	return
}

func (ui *HTMLInner) JawsUpdate(e *core.Element) {
	e.SetInner(ui.HTMLGetter.JawsGetHTML(e))
}
