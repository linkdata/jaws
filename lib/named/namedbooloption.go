package named

import (
	"html"
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/htmlio"
)

// namedBoolOption is an internal UI wrapper used by NamedBoolArray.JawsContains.
// It intentionally stays unexported; public option widgets live in package ui.
type namedBoolOption struct {
	*Bool
}

func (opt namedBoolOption) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	elem.Tag(opt.Bool)
	attrs := elem.ApplyParams(params)
	valattr := template.HTMLAttr(`value="` + html.EscapeString(opt.Name()) + `"`) // #nosec G203
	attrs = append(attrs, valattr)
	if opt.Checked() {
		attrs = append(attrs, "selected")
	}
	return htmlio.WriteHTMLInner(w, elem.Jid(), "option", "", opt.JawsGetHTML(elem), attrs...)
}

func (opt namedBoolOption) JawsUpdate(elem *jaws.Element) {
	if opt.Checked() {
		elem.SetAttr("selected", "")
	} else {
		elem.RemoveAttr("selected")
	}
}
