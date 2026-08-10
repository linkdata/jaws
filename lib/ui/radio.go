package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
)

// Radio renders an HTML radio input bound to a bool setter.
//
// A Radio value must back at most one live [jaws.Element]. Construct distinct
// Radio values over the same setter to render one bound value more than once.
//
// Each Radio binds one independent boolean. When separately bound Radio widgets
// belong to the same native HTML group, selecting one unchecks its peers without
// sending input events for those peers, so their Go values are not changed. Use
// [RequestWriter.RadioGroup] with a single-select [named.BoolArray] whose
// [named.Bool.Name] values are distinct, or custom setters that clear peers as
// one synchronized state change and then dirty every changed binding, when Go
// state must be mutually exclusive.
type Radio struct{ InputBool }

// NewRadio returns a radio input widget bound to g.
//
// For writable use, g must provide the setter-derived dirty target described by [Input].
// See [Radio] for grouping semantics.
func NewRadio(g bind.Setter[bool]) *Radio { return &Radio{InputBool{Setter: g}} }

// JawsRender renders ui as an HTML radio input.
func (u *Radio) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.renderBoolInput(elem, w, "radio", params...)
}

// Radio renders an HTML radio input.
//
// See [Radio] for grouping semantics.
func (rw RequestWriter) Radio(value any, params ...any) error {
	return rw.NewUI(NewRadio(bind.MakeSetter[bool](value)), params...)
}
