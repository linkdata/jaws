package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Text struct{ InputText }

func NewText(vp pkg.Setter[string]) *Text { return &Text{InputText{Setter: vp}} }
func (ui *Text) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "text", params...)
}
