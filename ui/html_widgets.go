package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsbind"
	"github.com/linkdata/jaws/jawshtml"
)

// HTMLInner is a reusable base for widgets that render as `<tag>inner</tag>`.
type HTMLInner struct {
	HTMLGetter jawsbind.HTMLGetter
}

func (ui *HTMLInner) renderInner(e *jaws.Element, w io.Writer, htmlTag, htmlType string, params []any) (err error) {
	if _, err = e.ApplyGetter(ui.HTMLGetter); err == nil {
		err = jawshtml.WriteHTMLInner(w, e.Jid(), htmlTag, htmlType, ui.HTMLGetter.JawsGetHTML(e), e.ApplyParams(params)...)
	}
	return
}

func (ui *HTMLInner) JawsUpdate(e *jaws.Element) {
	e.SetInner(ui.HTMLGetter.JawsGetHTML(e))
}
