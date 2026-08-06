package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/named"
)

// Select renders a single-selection HTML select element.
//
// Its handler supplies the options and reads and writes one selected option name.
// The handler's dynamic value defines Select's identity and must be comparable
// and equal to itself. Rebuilding with an equal handler lets a parent retain its
// live Element. Keep application state containing a slice, map, or function
// behind a stable pointer and synchronize access to it.
//
// Equal Select values may back multiple live [jaws.Element] values in one
// [jaws.Request] when the handler is safe for all calls and each option UI value
// reused across those Elements supports multiple live Elements. Use Select as a
// value; taking its address changes identity and is unsupported.
//
// A nil-interface handler makes [Select.JawsInput] a no-op and panics when
// rendering or updating requests options. A typed-nil handler is called normally
// and must tolerate its nil receiver.
//
// Select supports one selected option; a multiple select is unsupported.
type Select struct {
	handler named.SelectHandler
}

var (
	_ jaws.UI           = Select{}
	_ jaws.InputHandler = Select{}
)

// NewSelect returns a single-selection Select backed by handler.
func NewSelect(handler named.SelectHandler) Select {
	return Select{handler: handler}
}

// JawsRender renders u as an HTML select element.
//
// On success, it queues the selected value after the options.
func (u Select) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	err = u.container().render(elem, w, params, func() { u.applyValue(elem) })
	return
}

// JawsUpdate reconciles the child options and then queues the selected value.
//
// After a non-contended reconciliation returns, it queues the selected value.
// State contention suppresses both operations.
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
// A nil-interface handler is a no-op; a typed-nil handler is called normally.
// If no render-derived dirty tag is available, Select performs no additional
// dirtying after calling the handler.
func (u Select) JawsInput(elem *jaws.Element, value string) (err error) {
	if u.handler != nil {
		err = applyDirty(containerDirtyTag(elem), elem, u.handler.JawsSet(elem, value))
	}
	return
}

// Select renders a single-selection HTML select element.
//
// HTML attribute params are applied to the select element, but the multiple
// attribute is unsupported because Select stores one selected option name.
func (rw RequestWriter) Select(handler named.SelectHandler, params ...any) error {
	return rw.NewUI(NewSelect(handler), params...)
}
