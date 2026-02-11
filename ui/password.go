package ui

import (
	"io"

	"github.com/linkdata/jaws/core"
)

type Password struct{ InputText }

func NewPassword(g core.Setter[string]) *Password { return &Password{InputText{Setter: g}} }
func (ui *Password) JawsRender(e *core.Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "password", params...)
}
