package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
)

type Text struct{ InputText }

func NewText(vp bind.Setter[string]) *Text { return &Text{InputText{Setter: vp}} }
func (rw RequestWriter) Text(value any, params ...any) error {
	return rw.UI(NewText(bind.MakeSetter[string](value)), params...)
}

func (ui *Text) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "text", params...)
}
