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
// Register does not call the updater's [jaws.Renderer.JawsRender]. The updater
// must support [jaws.Updater.JawsUpdate] without render-time initialization. A
// [Template] returned by [NewTemplate] does not; [Container], [Tbody], and [Select] do.
//
// Register does not forward event-handler methods. [RequestWriter.Register]
// automatically attaches handler methods implemented by its updater.
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

// Register creates an update-only Element and returns its [jid.Jid].
//
// The updater is also a tag for dynamic updates. Additional tags may be provided
// in params.
// If updater also implements an event handler interface, it receives matching
// events after handlers provided in params have had a chance to handle them.
// The updater's [jaws.Updater.JawsUpdate] method will be called immediately to
// ensure the initial rendering is correct.
//
// A surrounding [Template] owns the Element and unregisters it when its content
// is replaced. Otherwise ordinary DOM-removal handling unregisters it; an Element
// whose Jid never reaches the DOM remains until explicitly removed or its
// [jaws.Request] ends.
//
// Register does not call the updater's [jaws.Renderer.JawsRender]; see [Register]
// for updater constraints. It automatically attaches event-handler methods
// implemented by updater.
//
// A Select registered this way retains no handler-derived tag for its own post-set
// dirtying. Any handler-initiated dirtying still occurs, and separately registered
// tags remain registered. Use [RequestWriter.Select] when Select should register and
// use a usable tag exposed by its handler. Typed input widgets require their ordinary
// NewUI or RequestWriter rendering path.
//
// The returned Jid is suitable for including as an HTML id attribute:
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
