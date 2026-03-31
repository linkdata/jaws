package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsbind"
)

type Password struct{ InputText }

func NewPassword(g jawsbind.Setter[string]) *Password { return &Password{InputText{Setter: g}} }
func (rw RequestWriter) Password(value any, params ...any) error {
	return rw.UI(NewPassword(jawsbind.MakeSetter[string](value)), params...)
}

func (ui *Password) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "password", params...)
}
