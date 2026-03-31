package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsbind"
)

type Label struct{ HTMLInner }

func NewLabel(innerHTML jawsbind.HTMLGetter) *Label { return &Label{HTMLInner{HTMLGetter: innerHTML}} }
func (rw RequestWriter) Label(innerHTML any, params ...any) error {
	return rw.UI(NewLabel(jawsbind.MakeHTMLGetter(innerHTML)), params...)
}

func (ui *Label) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "label", "", params)
}
