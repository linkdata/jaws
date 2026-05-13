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
func (u *Select) JawsUpdate(elem *jaws.Element) {
	// jawsbind.Setter[T] includes jawsbind.Getter[T]
	elem.SetValue(u.ContainerHelper.Container.(bind.Setter[string]).JawsGet(elem))
	u.UpdateContainer(elem)
}

// JawsInput stores a browser-side select value.
func (u *Select) JawsInput(elem *jaws.Element, value string) (err error) {
	err = applyDirty(u.Tag, elem, u.ContainerHelper.Container.(bind.Setter[string]).JawsSet(elem, value))
	return
}

// Select renders an HTML select element.
func (rw RequestWriter) Select(sh named.SelectHandler, params ...any) error {
	return rw.UI(NewSelect(sh), params...)
}
