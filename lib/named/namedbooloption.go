package named

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/htmlio"
)

// RenderBoolOption renders nb as an HTML <option> element into w. It is the
// single source of <option> markup shared by BoolArray's options and by
// ui.Option, so attribute/escaping behavior cannot drift between them.
func RenderBoolOption(elem *jaws.Element, w io.Writer, nb *Bool, params []any) error {
	elem.Tag(nb)
	attrs := elem.ApplyParams(params)
	attrs = append(attrs, htmlio.Attr("value", nb.Name()))
	if nb.Checked() {
		attrs = append(attrs, "selected")
	}
	return htmlio.WriteHTMLInner(w, elem.Jid(), "option", "", nb.JawsGetHTML(elem), attrs...)
}

// UpdateBoolOption updates a rendered <option>'s live selected state to match nb.
// It is the single source of the option's update behavior.
func UpdateBoolOption(elem *jaws.Element, nb *Bool) {
	if nb.Checked() {
		elem.SetValue("true")
	} else {
		elem.SetValue("false")
	}
}

// namedBoolOption is an internal UI wrapper used by BoolArray.JawsContains.
// It intentionally stays unexported; public option widgets live in package ui.
type namedBoolOption struct {
	*Bool
}

func (opt namedBoolOption) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return RenderBoolOption(elem, w, opt.Bool, params)
}

func (opt namedBoolOption) JawsUpdate(elem *jaws.Element) {
	UpdateBoolOption(elem, opt.Bool)
}
