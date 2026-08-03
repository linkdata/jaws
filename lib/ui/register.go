package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/jid"
)

// Register is an update-only widget that renders no HTML; it exists so its
// embedded [jaws.Updater] receives dynamic updates.
//
// One Register value may back multiple live [jaws.Element] values only when its
// Updater supports those calls without retaining Element-specific state on a
// shared value.
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
// A surrounding [Template] owns the Element and unregisters it when the template next
// replaces its content, so repeated updates do not accumulate registrations. With no
// template owner — rendered through a [RequestWriter] a caller built itself — cleanup
// falls to the ordinary DOM-removal handling: the browser reports the JaWS ids it
// removes when an ancestor's content is replaced, and [jaws.Element.Remove] unregisters
// a managed child outright. Only an Element no removal ever reports, such as one whose
// returned [jid.Jid] never becomes an element id, necessarily stays registered until the
// [jaws.Request] ends.
//
// Register does not call [jaws.Renderer.JawsRender]. The updater must therefore
// be ready for JawsUpdate and event handling without render-time initialization.
// In particular, the standard input widgets and [Select] initialize their dirty
// targets while rendering; register them with [RequestWriter.NewUI] or their
// RequestWriter helper when they need to handle input events.
//
// Returns a [jid.Jid], suitable for including as an HTML id attribute:
//
//	<div id="{{$.Register .MyUpdater}}">...</div>
func (rw RequestWriter) Register(updater jaws.Updater, params ...any) jid.Jid {
	// Create and report the Element rather than going through RequestWriter.NewUI, so
	// a surrounding Template owns it without anything being rendered: NewUI calls
	// JawsRender, which appends a debug comment when Jaws.Debug is set, and the
	// documented usage puts the returned Jid inside an attribute
	// (<div id="{{$.Register .X}}">), where that comment would corrupt the markup.
	elem := rw.NewElement(Register{Updater: updater})
	rw.trackElement(elem)
	elem.Tag(updater)
	// The wrapping Register element's UI is not the updater, so events reach the
	// updater only through the element's handler list, not the elem.UI() fallback.
	switch updater.(type) {
	case jaws.InputHandler, jaws.ClickHandler, jaws.ContextMenuHandler:
		elem.AddHandlers(updater)
	}
	elem.ApplyParams(params)
	updater.JawsUpdate(elem)
	elem.Freeze()
	return elem.Jid()
}
