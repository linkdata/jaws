package ui

import (
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/jid"
)

// registerUI adapts a render-independent updater to [jaws.UI]. The surrounding
// template renders the Element's DOM node and places its JaWS ID on that node.
type registerUI struct{ jaws.Updater }

func (registerUI) JawsRender(*jaws.Element, io.Writer, []any) error {
	return nil
}

// Register binds updater to a template-authored HTML element.
//
// The returned [jid.Jid] must be the element's id. Register never calls
// [jaws.Renderer.JawsRender], so updater must work without render-time
// initialization. Prefer [RequestWriter.NewUI] or a widget helper when the widget
// can render its own element.
//
// Register tags the Element with updater, applies tag and supported event-handler
// params, attaches click and context-menu methods implemented by updater, and
// invokes [jaws.Updater.JawsUpdate] once for the initial browser state. A
// registered [Select] also receives browser input through its input handler.
// Updater handlers are tried only after applicable param handlers return
// [jaws.ErrEventUnhandled]. HTML attribute params have no effect; write
// attributes in the template.
//
// The updater must be non-nil, comparable at runtime, equal to itself, and usable
// as a tag. A typed nil is invoked normally and must tolerate its nil receiver.
// The same updater may back multiple live Elements only when it supports that use
// without retaining Element-specific state on the shared value. If shared across
// requests, it must be safe for concurrent use.
//
// A surrounding [Template] owns and cleans up the registered Element. Outside a
// Template, it remains registered until explicitly deleted, DOM removal is
// reported, or its [jaws.Request] ends; always emit the returned Jid.
//
// [Container], [Tbody], and [Select] support registration, though ordinary
// rendering is preferable. A registered Select omits its handler-derived tag for
// post-input dirtying. Template-authored input and textarea elements,
// contenteditable elements, the typed input widgets, [Number], [Range], [JsVar],
// and a [Template] with a non-empty OuterHTMLTag require ordinary rendering and
// are not supported through Register.
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
	elem := rw.NewElement(registerUI{Updater: updater})
	rw.trackElement(elem)
	elem.Tag(updater)
	// The registerUI Element's UI is not the updater, so events reach the
	// updater only through the element's handler list, not the elem.UI() fallback.
	// InputHandler is retained for the documented registered Select path; the
	// standard input widgets require ordinary rendering.
	switch updater.(type) {
	case jaws.InputHandler, jaws.ClickHandler, jaws.ContextMenuHandler:
		elem.AddHandlers(updater)
	}
	elem.ApplyParams(params)
	updater.JawsUpdate(elem)
	elem.Freeze()
	return elem.Jid()
}
