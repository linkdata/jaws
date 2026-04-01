package namedbool

import (
	"html"
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/htmlio"
)

// namedBoolOption is an internal UI wrapper used by NamedBoolArray.JawsContains.
// It intentionally stays unexported; public option widgets live in package ui.
type namedBoolOption struct {
	*NamedBool
}

func (ui namedBoolOption) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	e.Tag(ui.NamedBool)
	attrs := e.ApplyParams(params)
	valattr := template.HTMLAttr(`value="` + html.EscapeString(ui.Name()) + `"`) // #nosec G203
	attrs = append(attrs, valattr)
	if ui.Checked() {
		attrs = append(attrs, "selected")
	}
	return htmlio.WriteHTMLInner(w, e.Jid(), "option", "", ui.JawsGetHTML(e), attrs...)
}

func (ui namedBoolOption) JawsUpdate(e *jaws.Element) {
	if ui.Checked() {
		e.SetAttr("selected", "")
	} else {
		e.RemoveAttr("selected")
	}
}
