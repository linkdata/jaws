package ui

import (
	"encoding/json"
	"io"
	"slices"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/named"
)

// MultiSelect renders an HTML select element with multiple selection enabled.
//
// Its handler supplies the options and all selected values. Option values must
// be non-empty and distinct. The standard handler is [named.BoolArray]
// constructed with named.NewBoolArray(true). An empty or nil selection clears
// every option; values without a matching option select nothing.
//
// Like [Select], MultiSelect is an immutable definition used as a value. Its
// handler must be comparable and equal to itself. Equal definitions may back
// multiple live Elements when the handler and option widgets support that use.
// Keep shared application state synchronized behind stable pointers.
//
// Native form reset does not update Go state. Reset the authoritative selection
// from a JaWS-handled button with type="button", then dirty its tag.
// The complete browser input message must fit the 32 KiB inbound limit; larger
// selections close the Request connection.
type MultiSelect struct {
	handler named.MultiSelectHandler
}

var (
	_ jaws.UI           = MultiSelect{}
	_ jaws.InputHandler = MultiSelect{}
)

// NewMultiSelect returns a MultiSelect backed by handler.
// Use named.NewBoolArray(true) for a [named.BoolArray] handler.
func NewMultiSelect(handler named.MultiSelectHandler) MultiSelect {
	return MultiSelect{handler: handler}
}

// JawsRender renders the options and queues their complete selected state.
func (u MultiSelect) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return u.container().render(elem, w, append([]any{"multiple"}, params...), func() { u.applyValues(elem) })
}

// JawsUpdate reconciles options before queuing their complete selected state.
// Missing, foreign, or in-progress state suppresses both operations.
func (u MultiSelect) JawsUpdate(elem *jaws.Element) {
	if u.container().update(elem) {
		u.applyValues(elem)
	}
}

func (u MultiSelect) applyValues(elem *jaws.Element) {
	values := u.handler.JawsGetValues(elem)
	if values == nil {
		values = []string{}
	}
	data, _ := json.Marshal(values) // A string slice is always JSON encodable.
	elem.SetValue(string(data))
}

func (u MultiSelect) container() Container {
	return NewContainer("select", u.handler)
}

// JawsInput replaces the selected values from a browser JSON array of strings.
// Malformed input is ignored without calling the handler. Every proposal
// reconciles the originating Element, including rejected or unchanged values.
// A nil-interface handler is a no-op; a typed-nil handler is called normally.
func (u MultiSelect) JawsInput(elem *jaws.Element, value string) (err error) {
	if u.handler != nil {
		var values []string
		if json.Unmarshal([]byte(value), &values) == nil && values != nil && !slices.Contains(values, "") {
			err = applyDirty(containerDirtyTag(elem), elem, u.handler.JawsSetValues(elem, values))
		}
		elem.Dirty(elem)
	}
	return
}

// MultiSelect renders an HTML select element with multiple selection enabled.
// See [MultiSelect] for handler requirements and native reset semantics.
func (rw RequestWriter) MultiSelect(handler named.MultiSelectHandler, params ...any) error {
	return rw.NewUI(NewMultiSelect(handler), params...)
}
