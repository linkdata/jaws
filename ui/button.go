package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Button struct{ HTMLInner }

func NewButton(innerHTML pkg.HTMLGetter) *Button { return &Button{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Button) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "button", "button", params)
}
