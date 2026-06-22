package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/named"
)

// Select renders a single-selection HTML select element.
//
// The widget stores one selected option name through a [named.SelectHandler].
// Render params are written as supplied, but a multiple select is not
// supported by the JaWS select value contract.
type Select struct {
	ContainerHelper
}

// NewSelect returns a single-selection select widget backed by sh.
//
// The widget reads and writes one selected option name through sh.
func NewSelect(sh named.SelectHandler) *Select {
	return &Select{ContainerHelper: NewContainerHelper(sh)}
}

// JawsRender renders ui as an HTML select element.
func (u *Select) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.RenderContainer(elem, w, "select", params)
}

// JawsUpdate updates the selected value and child options.
//
// Unlike the typed inputs, it re-sends the select value on every update with no
// dedup against a last value, so mark the element dirty only when the value or
// options actually changed.
func (u *Select) JawsUpdate(elem *jaws.Element) {
	u.UpdateContainer(elem)
	// The selected value is only set when the Container is a bind.Setter of string;
	// the child options are always updated. NewSelect always supplies a
	// named.SelectHandler (a bind.Setter of string), so this guard (and the one in
	// JawsInput) only takes effect if Container is later reassigned to a plain
	// jaws.Container.
	if setter, ok := u.ContainerHelper.Container.(bind.Setter[string]); ok {
		// Set the live value after reconciling options, so a newly selected
		// option exists in the browser before the select value is assigned.
		elem.SetValue(setter.JawsGet(elem))
	}
}

// JawsInput stores one browser-side selected option name.
//
// The input is ignored (returning a nil error) when the Container is not a
// [bind.Setter] of string.
func (u *Select) JawsInput(elem *jaws.Element, value string) (err error) {
	if setter, ok := u.ContainerHelper.Container.(bind.Setter[string]); ok {
		err = applyDirty(u.tag, elem, setter.JawsSet(elem, value))
	}
	return
}

// Select renders a single-selection HTML select element.
//
// Params are rendered as supplied. Passing a multiple attribute is unsupported
// because the widget stores one selected option name.
func (rw RequestWriter) Select(sh named.SelectHandler, params ...any) error {
	return rw.NewUI(NewSelect(sh), params...)
}
