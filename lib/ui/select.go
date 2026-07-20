package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/named"
)

// Select renders a single-selection HTML select element.
//
// A Select value must back at most one live [jaws.Element]. Construct distinct
// Select values over the same [named.SelectHandler] to render one selection more
// than once.
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

// JawsRender renders ui as an HTML select element and applies its selected value.
//
// The value is queued even when the option markup may already represent it:
// [named.SelectHandler] permits custom option renderers, so Select cannot know
// whether that markup agrees with the handler's getter.
func (u *Select) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	if err = u.RenderContainer(elem, w, "select", params); err == nil {
		u.applyValue(elem)
	}
	return
}

// JawsUpdate updates the selected value and child options.
//
// Unlike the typed inputs, it re-sends the select value on every update with no
// dedup against a last value, so mark the element dirty only when the value or
// options actually changed.
func (u *Select) JawsUpdate(elem *jaws.Element) {
	u.UpdateContainer(elem)
	u.applyValue(elem)
}

func (u *Select) applyValue(elem *jaws.Element) {
	// NewSelect always supplies a named.SelectHandler (a bind.Setter of string),
	// so this guard (and the one in JawsInput) only takes effect if Container is
	// later reassigned to a plain jaws.Container.
	if setter, ok := u.ContainerHelper.Container.(bind.Setter[string]); ok {
		// JawsRender and JawsUpdate call this only after rendering or reconciling
		// options, so the options exist before the select value is assigned.
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
