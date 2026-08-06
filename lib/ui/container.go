package ui

import (
	"io"

	"github.com/linkdata/jaws"
)

// Container renders an HTML element around a dynamic child collection.
//
// Container is an immutable definition whose mutable reconciliation state is
// kept separately on each [jaws.Element]. Equal Container values can therefore
// back multiple live Elements within one [jaws.Request], provided the child
// provider is safe for every call.
//
// The dynamic value of the child provider must be comparable at runtime and
// equal to itself. Rebuilding a Container with the same outer tag and the same
// stable provider pointer produces an equal value, allowing a containing widget
// to reuse the existing Element. When a provider contains a slice, map, function
// or other runtime-incomparable value, pass a stable pointer to it instead.
// Application state behind that pointer may change with appropriate
// synchronization, but the Container definition itself must not change.
//
// Reconciliation is parent-local and updates direct children only. Reordering equal
// direct children preserves their Elements and complete nested subtrees. Each nested
// Container, [Tbody], or [Select] whose children changed must receive its own dirty or
// update pass; updating an outer Element is not recursive. Moving a child definition
// between different parents is reparenting: the old Element is removed and a new one
// is rendered under the new parent.
//
// Use Container as a value. Taking its address is unsupported because pointer
// identity prevents independently rebuilt definitions from comparing equal.
//
// The zero value, and a Container constructed with a nil-interface child
// provider, panic when rendering or updating tries to obtain children. A typed
// nil provider is called normally and must tolerate its nil receiver itself.
type Container struct {
	outerHTMLTag string
	children     jaws.Container
}

var _ jaws.UI = Container{}

// NewContainer returns a container widget that renders children inside outerHTMLTag.
//
// The returned definition is immutable. Reuse the same stable child-provider
// pointer when rebuilding an equal Container value.
func NewContainer(outerHTMLTag string, children jaws.Container) Container {
	return Container{outerHTMLTag: outerHTMLTag, children: children}
}

// JawsRender renders u as its configured container element.
//
// It claims elem's widget state slot before applying getters or params, calling
// the child provider, or writing output. An occupied slot returns
// [jaws.ErrElementStateClaimed] without performing those side effects.
func (u Container) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.render(elem, w, params, nil)
}

// JawsUpdate updates u's child collection.
//
// It claims an unoccupied widget state slot lazily for update-only use. A foreign
// or typed-nil state, an in-progress render, or a lost concurrent claim reports
// [jaws.ErrElementStateClaimed] through [jaws.Request.MustLog] without calling the
// child provider or queuing browser work. [jaws.Request.MustLog] panics when no
// [jaws.Jaws.Logger] is configured.
func (u Container) JawsUpdate(elem *jaws.Element) {
	u.update(elem)
}

// Container renders children inside outerHTMLTag.
func (rw RequestWriter) Container(outerHTMLTag string, children jaws.Container, params ...any) error {
	return rw.NewUI(NewContainer(outerHTMLTag, children), params...)
}
