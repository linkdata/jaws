package ui

import "github.com/linkdata/jaws"

// Tbody renders an HTML tbody containing dynamic child rows.
//
// [NewTbody] configures its embedded [Container] for tbody. Replacing that
// Container is unsupported. Tbody otherwise follows Container's identity,
// reconciliation, multiplicity, and nil-provider contracts. Treat it as
// immutable after use and use it as a value; taking its address changes identity
// and is unsupported.
type Tbody struct {
	Container
}

var _ jaws.UI = Tbody{}

// NewTbody returns a Tbody that renders children as table rows.
func NewTbody(children jaws.Container) Tbody {
	return Tbody{Container: NewContainer("tbody", children)}
}

// Tbody renders children in an HTML tbody element.
func (rw RequestWriter) Tbody(children jaws.Container, params ...any) error {
	return rw.NewUI(NewTbody(children), params...)
}
