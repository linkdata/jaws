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
//
// Register does not call the embedded updater's [jaws.Renderer.JawsRender]. An updater
// that requires render-only initialization therefore cannot work here: a wrapped
// [Template] reports [ErrElementStateUnclaimed] instead of updating, through
// [jaws.Request.MustLog], which panics when no [jaws.Jaws.Logger] is configured. An
// unwrapped Template is fine, since its updates are a documented no-op.
//
// [Container], [Tbody] and [Select] explicitly support update-only use. Their first
// update lazily claims per-Element container state rather than relying on render-time
// initialization. For Select that state has no render-time dirty tag. Only
// [RequestWriter.Register] delivers an updater's event handlers, because Register embeds
// [jaws.Updater] and promotes no handler methods of its own.
type Register struct{ jaws.Updater }

// NewRegister returns an update-only widget that invokes updater during updates.
//
// See [Register] for why a wrapped [Template] cannot be used as the updater.
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
// RequestWriter.Register does not call the updater's [jaws.Renderer.JawsRender]. The
// updater must therefore be ready for JawsUpdate and event handling without render-time
// initialization. A wrapped [Template] reports [ErrElementStateUnclaimed] through
// [jaws.Request.MustLog] rather than updating. An unwrapped Template works — its updates
// are a documented no-op — and this helper is the only Register form that also delivers
// a Template's click, input and context-menu handlers, since it adds the concrete updater
// to the Element's handler list.
//
// [Container], [Tbody] and [Select] lazily claim their per-Element container state on the
// immediate update, so their reconciliation works through this update-only path. No
// render-time getter ran, so the state has a nil dirty tag. A registered Select receives
// input through this helper and still calls its handler, but applies the result with that
// nil tag; use [RequestWriter.NewUI] or [RequestWriter.Select] when input must dirty the
// handler's render-time dependency. The typed input widgets require their ordinary render
// initialization and should likewise be registered through their NewUI or RequestWriter
// helpers when they need to handle input events.
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
