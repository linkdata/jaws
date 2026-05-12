package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

type Password struct{ InputText }

// NewPassword returns a password input widget bound to g.
func NewPassword(g bind.Setter[string]) *Password { return &Password{InputText{Setter: g}} }

func (ui *Password) JawsRender(e *jaws.Element, w io.Writer, params []any) error {
	return ui.renderStringInput(e, w, "password", params...)
}

func (rw RequestWriter) Password(value any, params ...any) error {
	return rw.UI(NewPassword(bind.MakeSetter[string](value)), params...)
}
