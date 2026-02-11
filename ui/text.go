package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Text struct{ InputText }

func NewText(vp core.Setter[string]) *Text { return &Text{InputText{Setter: vp}} }
func (ui *Text) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "text", params...)
}
