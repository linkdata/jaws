package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/named"
)

// Select renders an HTML select element backed by a [named.SelectHandler].
type Select struct {
	ContainerHelper
}

// NewSelect returns a select widget backed by sh.
func NewSelect(sh named.SelectHandler) *Select {
	return &Select{ContainerHelper: NewContainerHelper(sh)}
}

// JawsRender renders ui as an HTML select element.
func (u *Select) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.RenderContainer(elem, w, "select", params)
}

// JawsUpdate updates the selected value and child options.
//
// The selected value is only updated when the Container is a [bind.Setter] of
// string; the child options are always updated.
func (u *Select) JawsUpdate(elem *jaws.Element) {
	if setter, ok := u.ContainerHelper.Container.(bind.Setter[string]); ok {
		elem.SetValue(setter.JawsGet(elem))
	}
	u.UpdateContainer(elem)
}

// JawsInput stores a browser-side select value.
//
// The input is ignored (returning a nil error) when the Container is not a
// [bind.Setter] of string.
func (u *Select) JawsInput(elem *jaws.Element, value string) (err error) {
	if setter, ok := u.ContainerHelper.Container.(bind.Setter[string]); ok {
		err = applyDirty(u.tag, elem, setter.JawsSet(elem, value))
	}
	return
}

// Select renders an HTML select element.
func (rw RequestWriter) Select(sh named.SelectHandler, params ...any) error {
	return rw.UI(NewSelect(sh), params...)
}
