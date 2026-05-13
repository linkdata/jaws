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

func (u *HTMLInner) renderInner(elem *jaws.Element, w io.Writer, htmlTag, htmlType string, params []any) (err error) {
	var getterAttrs []template.HTMLAttr
	if _, getterAttrs, err = elem.ApplyGetter(u.HTMLGetter); err == nil {
		attrs := append(elem.ApplyParams(params), getterAttrs...)
		err = htmlio.WriteHTMLInner(w, elem.Jid(), htmlTag, htmlType, u.HTMLGetter.JawsGetHTML(elem), attrs...)
	}
	return
}

// JawsUpdate updates the rendered inner HTML.
func (u *HTMLInner) JawsUpdate(elem *jaws.Element) {
	elem.SetInner(u.HTMLGetter.JawsGetHTML(elem))
}
