package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/jawsbind"
)

type Text struct{ InputText }

func NewText(vp jawsbind.Setter[string]) *Text { return &Text{InputText{Setter: vp}} }
func (rw RequestWriter) Text(value any, params ...any) error {
	return rw.UI(NewText(jawsbind.MakeSetter[string](value)), params...)
}

func (ui *Text) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "text", params...)
}
