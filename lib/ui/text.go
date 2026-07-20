package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Text renders an HTML text input bound to a string setter.
//
// A Text value must back at most one live [jaws.Element]. Construct distinct
// Text values over the same setter to render one bound value more than once.
type Text struct{ InputText }

// NewText returns a text input widget bound to g.
func NewText(g bind.Setter[string]) *Text { return &Text{InputText{Setter: g}} }

// JawsRender renders ui as an HTML text input.
func (u *Text) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderStringInput(elem, w, "text", params...)
}

// Text renders an HTML text input.
func (rw RequestWriter) Text(value any, params ...any) error {
	return rw.NewUI(NewText(bind.MakeSetter[string](value)), params...)
}
