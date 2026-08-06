package ui

import "github.com/linkdata/jaws"

// Tbody renders an HTML tbody containing dynamic child rows.
//
// Tbody embeds the [Container] definition configured by [NewTbody]. Its mutable
// reconciliation state is kept separately on each [jaws.Element]. Equal Tbody
// values can therefore back multiple live Elements within one [jaws.Request],
// provided the row provider is safe for every call.
//
// The dynamic value of the row provider must be comparable at runtime and equal
// to itself. Rebuilding a Tbody with the same stable provider pointer produces an
// equal value, allowing a containing widget to reuse the existing Element. When
// a provider contains a slice, map, function or other runtime-incomparable value,
// pass a stable pointer to it instead. Application state behind that pointer may
// change with appropriate synchronization, but the Tbody definition itself must
// not change.
//
// Use Tbody as a value. Taking its address is unsupported because pointer identity
// prevents independently rebuilt definitions from comparing equal.
//
// The zero value, and a Tbody constructed with a nil-interface row provider,
// panic when rendering or updating tries to obtain rows. A typed nil provider is
// called normally and must tolerate its nil receiver itself.
type Tbody struct {
	Container
}

var _ jaws.UI = Tbody{}

// NewTbody returns a tbody widget that renders and updates children as table rows.
//
// The returned definition is immutable. Reuse the same stable row-provider
// pointer when rebuilding an equal Tbody value.
func NewTbody(children jaws.Container) Tbody {
	return Tbody{Container: NewContainer("tbody", children)}
}

// Tbody renders children in an HTML tbody element.
func (rw RequestWriter) Tbody(children jaws.Container, params ...any) error {
	return rw.NewUI(NewTbody(children), params...)
}
