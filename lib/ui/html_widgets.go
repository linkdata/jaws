package ui

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

// HTMLInner is a reusable base for widgets that render as `<tag>inner</tag>`.
type HTMLInner struct {
	// HTMLGetter returns the trusted inner HTML to render and update.
	HTMLGetter bind.HTMLGetter
}

func (ui *HTMLInner) renderInner(e *jaws.Element, w io.Writer, htmlTag, htmlType string, params []any) (err error) {
	var getterAttrs []template.HTMLAttr
	if _, getterAttrs, err = e.ApplyGetter(ui.HTMLGetter); err == nil {
		attrs := append(e.ApplyParams(params), getterAttrs...)
		err = htmlio.WriteHTMLInner(w, e.Jid(), htmlTag, htmlType, ui.HTMLGetter.JawsGetHTML(e), attrs...)
	}
	return
}

// JawsUpdate updates the rendered inner HTML.
func (ui *HTMLInner) JawsUpdate(e *jaws.Element) {
	e.SetInner(ui.HTMLGetter.JawsGetHTML(e))
}
