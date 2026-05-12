package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Text renders an HTML text input bound to a string setter.
type Text struct{ InputText }

// NewText returns a text input widget bound to vp.
func NewText(vp bind.Setter[string]) *Text { return &Text{InputText{Setter: vp}} }

// JawsRender renders ui as an HTML text input.
func (ui *Text) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "text", params...)
}

// Text renders an HTML text input.
func (rw RequestWriter) Text(value any, params ...any) error {
	return rw.UI(NewText(bind.MakeSetter[string](value)), params...)
}
