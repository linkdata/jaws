package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Password renders an HTML password input bound to a string setter.
//
// A Password value must back at most one live [jaws.Element]. Construct distinct
// Password values over the same setter to render one bound value more than once.
type Password struct{ InputText }

// NewPassword returns a password input widget bound to g.
func NewPassword(g bind.Setter[string]) *Password { return &Password{InputText{Setter: g}} }

// JawsRender renders ui as an HTML password input.
func (u *Password) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderStringInput(elem, w, "password", params...)
}

// Password renders an HTML password input.
func (rw RequestWriter) Password(value any, params ...any) error {
	return rw.NewUI(NewPassword(bind.MakeSetter[string](value)), params...)
}
