package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Label struct{ HTMLInner }

func NewLabel(innerHTML pkg.HTMLGetter) *Label { return &Label{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Label) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "label", "", params)
}
