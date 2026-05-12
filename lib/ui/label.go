package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Label struct{ HTMLInner }

func NewLabel(innerHTML any) *Label {
	return &Label{HTMLInner{HTMLGetter: bind.MakeHTMLGetter(innerHTML)}}
}
func (rw RequestWriter) Label(innerHTML any, params ...any) error {
	return rw.UI(NewLabel(innerHTML), params...)
}

func (ui *Label) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "label", "", params)
}
