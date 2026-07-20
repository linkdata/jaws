package ui

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

// HTMLInner is a reusable base for widgets that render as `<tag>inner</tag>`.
//
// HTMLInner retains no Element-specific state. A widget embedding it may back
// multiple live [jaws.Element] values when its HTMLGetter is also safe for those
// Elements' render, update and event calls.
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
//
// Unlike the typed input widgets, which dedup against a stored last value,
// HTMLInner keeps no last-rendered value and re-sends the inner HTML on every
// update; mark the [jaws.Element] dirty only when the content has actually changed.
func (u *HTMLInner) JawsUpdate(elem *jaws.Element) {
	elem.SetInner(u.HTMLGetter.JawsGetHTML(elem))
}
