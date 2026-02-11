package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Li struct{ HTMLInner }

func NewLi(innerHTML pkg.HTMLGetter) *Li { return &Li{HTMLInner{HTMLGetter: innerHTML}} }
func (ui *Li) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "li", "", params)
}
