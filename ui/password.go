package ui

import (
	"io"

	pkg "github.com/linkdata/jaws/jaws"
)

type Password struct{ InputText }

func NewPassword(g pkg.Setter[string]) *Password { return &Password{InputText{Setter: g}} }
func (ui *Password) JawsRender(e *pkg.Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "password", params...)
}
