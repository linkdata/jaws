package ui

import (
	"io"

	"github.com/linkdata/jaws"
)

// Container renders an HTML element around a dynamic child collection.
//
// Its outer tag and child provider define its identity. The provider's dynamic
// value must be comparable and equal to itself. Rebuilding an equal Container
// lets a parent retain its live Element. Keep application state containing a
// slice, map, or function behind a stable pointer and synchronize access to it.
//
// Reconciliation affects direct children only. Reordering retained equal children
// preserves their Elements and nested subtrees. Nested containers whose children
// change need their own update. Child Element identity is scoped to its parent;
// moving a child definition between parents does not preserve its Element.
// Each child must render one direct DOM node carrying its Element's JaWS ID. Use
// [NewTemplate] for Template children so removal and ordering can target a wrapper.
//
// Equal Container values may back multiple live Elements in one [jaws.Request]
// when the provider is safe for all calls and each child UI value reused across
// those Elements supports multiple live Elements. Use Container as a value;
// taking its address changes identity and is unsupported.
//
// A typed-nil provider is called normally and must tolerate its nil receiver.
type Container struct {
	outerHTMLTag string
	children     jaws.Container
}

var _ jaws.UI = Container{}

// NewContainer returns a Container that renders children inside outerHTMLTag.
func NewContainer(outerHTMLTag string, children jaws.Container) Container {
	return Container{outerHTMLTag: outerHTMLTag, children: children}
}

// JawsRender renders u as its configured container element.
//
// If elem's widget state is occupied, JawsRender returns
// [jaws.ErrElementStateClaimed] without rendering.
func (u Container) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.render(elem, w, params, nil)
}

// JawsUpdate reconciles u's direct children.
//
// If elem's widget state cannot be used, JawsUpdate reports
// [jaws.ErrElementStateClaimed] through [jaws.Request.MustLog] without calling the
// provider or queuing browser work.
func (u Container) JawsUpdate(elem *jaws.Element) {
	u.update(elem)
}

// Container renders children inside outerHTMLTag.
func (rw RequestWriter) Container(outerHTMLTag string, children jaws.Container, params ...any) error {
	return rw.NewUI(NewContainer(outerHTMLTag, children), params...)
}
