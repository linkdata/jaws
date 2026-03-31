package ui

import (
	"io"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/bind"
)

type Password struct{ InputText }

func NewPassword(g bind.Setter[string]) *Password { return &Password{InputText{Setter: g}} }
func (rw RequestWriter) Password(value any, params ...any) error {
	return rw.UI(NewPassword(bind.MakeSetter[string](value)), params...)
}

func (ui *Password) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "password", params...)
}
