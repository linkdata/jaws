package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Text struct{ InputText }

// NewText returns a text input widget bound to vp.
func NewText(vp bind.Setter[string]) *Text { return &Text{InputText{Setter: vp}} }

func (ui *Text) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "text", params...)
}

func (rw RequestWriter) Text(value any, params ...any) error {
	return rw.UI(NewText(bind.MakeSetter[string](value)), params...)
}
