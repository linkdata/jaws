package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/named"
)

// Select renders a single-selection HTML select element.
//
// Select uses a [Container] definition configured for the select tag and its
// [named.SelectHandler]. The handler's dynamic value must be comparable at runtime,
// equal to itself, and stable across rebuilt Select values. Keep application data
// that contains a slice, map, or function behind a stable pointer and use that
// pointer as the handler.
//
// Equal Select values may back multiple live [jaws.Element] values in one
// [jaws.Request]. Each Element keeps its option-reconciliation state separately,
// while the handler remains the shared source of application state.
//
// Use Select as a value. A *Select uses pointer identity instead of definition
// equality and is unsupported.
//
// The zero value has a nil handler. Its [Select.JawsInput] is a no-op; rendering
// or updating it panics when the missing handler is called. A typed-nil handler
// is not treated as nil and receives calls normally, so its concrete methods must
// tolerate a nil receiver if it is to be used.
//
// The widget stores one selected option name through its handler. Render params
// are written as supplied, but a multiple select is not supported by the JaWS
// select value contract.
type Select struct {
	handler named.SelectHandler
}

var (
	_ jaws.UI           = Select{}
	_ jaws.InputHandler = Select{}
)

// NewSelect returns a single-selection select widget backed by handler.
//
// The widget reads and writes one selected option name through handler. The handler
// must satisfy the identity requirements documented by [Select] and must not be
// replaced after the widget is passed to JaWS.
func NewSelect(handler named.SelectHandler) Select {
	return Select{handler: handler}
}

// JawsRender renders u as an HTML select element and applies its selected value.
//
// The value is queued even when the option markup may already represent it:
// [named.SelectHandler] permits custom option renderers, so Select cannot know
// whether that markup agrees with the handler's getter.
func (u Select) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	err = u.container().render(elem, w, params, func() { u.applyValue(elem) })
	return
}

// JawsUpdate updates the child options and then the selected value.
//
// Unlike the typed inputs, it re-sends the select value on every successful
// update with no deduplication against a last value, so mark the element dirty
// only when the value or options actually changed. State contention prevents
// both option reconciliation and the value update.
func (u Select) JawsUpdate(elem *jaws.Element) {
	if u.container().update(elem) {
		u.applyValue(elem)
	}
}

func (u Select) applyValue(elem *jaws.Element) {
	// JawsRender and JawsUpdate call this only after rendering or reconciling
	// options, so the options exist before the select value is assigned.
	elem.SetValue(u.handler.JawsGet(elem))
}

func (u Select) container() Container {
	return NewContainer("select", u.handler)
}

// JawsInput stores one browser-side selected option name.
//
// JawsInput never claims missing Element state. When no render-time dirty tag is
// available, it still calls the handler and applies the result with a nil tag.
// A nil-interface handler is a no-op; a typed nil is dispatched normally.
func (u Select) JawsInput(elem *jaws.Element, value string) (err error) {
	if u.handler != nil {
		err = applyDirty(containerDirtyTag(elem), elem, u.handler.JawsSet(elem, value))
	}
	return
}

// Select renders a single-selection HTML select element.
//
// Params are rendered as supplied. Passing a multiple attribute is unsupported
// because the widget stores one selected option name.
func (rw RequestWriter) Select(handler named.SelectHandler, params ...any) error {
	return rw.NewUI(NewSelect(handler), params...)
}
