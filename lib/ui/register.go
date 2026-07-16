package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/jid"
)

// Register is an update-only widget that renders no HTML; it exists so its
// embedded [jaws.Updater] receives dynamic updates.
type Register struct{ jaws.Updater }

// NewRegister returns an update-only widget that invokes updater during updates.
func NewRegister(updater jaws.Updater) Register { return Register{Updater: updater} }

// JawsRender renders no HTML for update-only registration.
//
// It ignores params; to attach extra tags or event handlers, use
// [RequestWriter.Register], which applies them before the element is frozen.
func (u Register) JawsRender(elem *jaws.Element, w io.Writer, params []any) error {
	return nil
}

// Register creates a new Element with the given Updater as a tag
// for dynamic updates. Additional tags may be provided in params.
// If updater also implements an event handler interface, it receives matching
// events after handlers provided in params have had a chance to handle them.
// The updater's [jaws.Updater.JawsUpdate] method will be called immediately to
// ensure the initial rendering is correct.
//
// Returns a [jid.Jid], suitable for including as an HTML id attribute:
//
//	<div id="{{$.Register .MyUpdater}}">...</div>
func (rw RequestWriter) Register(updater jaws.Updater, params ...any) jid.Jid {
	elem := rw.NewElement(Register{Updater: updater})
	elem.Tag(updater)
	// The wrapping Register element's UI is not the updater, so events reach the
	// updater only through the element's handler list, not the elem.UI() fallback.
	switch updater.(type) {
	case jaws.InputHandler, jaws.ClickHandler, jaws.ContextMenuHandler:
		elem.AddHandlers(updater)
	}
	if r, ok := updater.(jaws.Renderer); ok {
		// Emit the initial update before rendering: the render path stores the
		// widget's last value and an input widget's JawsUpdate only emits on a
		// change, so the unconditional first update must run while the stored
		// value is still empty.
		updater.JawsUpdate(elem)
		// Run the real render with output discarded to assign the widget's dirty
		// tag (an input widget's tag is set only by [jaws.Element.ApplyGetter]
		// during render), so later input events dirty the bound tag. ApplyParams
		// runs inside JawsRender, so it is not repeated here.
		_ = r.JawsRender(elem, io.Discard, params)
	} else {
		elem.ApplyParams(params)
		updater.JawsUpdate(elem)
	}
	elem.Freeze()
	return elem.Jid()
}
