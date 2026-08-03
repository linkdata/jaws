package ui

import (
	"fmt"
	"html/template"
	"io"
	"strings"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/tag"
)

// Template references a Go [html/template] template to be rendered through JaWS.
//
// A Template retains no Element-specific state and may back multiple live
// [jaws.Element] values: the Elements created while it executes are tracked in the
// rendering Element's widget state slot (see [jaws.SetElementState]), not on the
// Template. Its Dot and any callbacks reached during execution are shared by those
// Elements and must be safe for their render, update and event calls.
//
// Template is a value widget and must be passed to JaWS as a value, normally the value
// returned by [NewTemplate]. Do not take its address: although Go gives *Template the
// value receiver's method set, a container would then key reuse on pointer identity
// instead of Template value equality.
//
// Like every [jaws.UI] value passed to [jaws.Request.NewElement], a Template must be
// comparable at runtime and equal to itself because the container widgets use UI values
// as map keys. A Dot holding a slice, map, func or NaN makes the Template unusable.
// [Handler] is the arbitrary-Dot exception; its private whole-page renderer is distinct
// from a Template widget and does not use the page dot as a tag.
//
// The state slot is claimed while rendering, so at most one Template may render a given
// Element and [Template.JawsUpdate] does nothing on an Element no Template rendered. The
// claim belongs to the Element rather than to the Template value that installed it, so a
// composite renderer may legitimately delegate JawsRender to one Template and JawsUpdate
// to another; and a claim survives a render error the delegating renderer handles, so a
// later update still runs.
//
// The OuterHTMLTag field identifies the generated wrapper element used for
// partial templates. If OuterHTMLTag is empty, the template is rendered without
// a generated wrapper. Name identifies the template to execute and Dot contains
// the data exposed to the template through the [With] structure constructed
// during rendering. Wrapped templates receive the JaWS ID and any HTML
// attributes supplied at render time through the [RequestWriter.Template]
// helper. The referenced template must be a partial template, not a full HTML
// document.
//
// Every Element a template creates through the [RequestWriter] it is given belongs to
// the Template that rendered it — the widget helpers, [RequestWriter.Register],
// [RequestWriter.RadioGroup] and a nested [RequestWriter.Template] alike. When
// [Template.JawsUpdate] replaces the wrapper's content, those Elements are
// unregistered along with the DOM that held them, and a nested widget's own Elements
// go with it.
//
// Ownership is recorded when an Element is created rather than after it renders, so
// one that never reaches the browser is reclaimed too: an Element whose render failed,
// or a radio Element left unrendered by a [RadioElement.Label] without its
// [RadioElement.Radio]. See [RequestWriter.RadioGroup] for the one attribution
// condition, which applies when a group's markup ends up in a different wrapper than
// the RadioGroup call.
//
// Template execution is best-effort rather than transactional. Template actions
// and nested JaWS helpers run as the template executes, so an execution error
// after partial output can leave already-written HTML, queued messages, domain
// mutations or other side effects in place. The tracked Elements created by the
// failed execution are unregistered. Treat such errors as application bugs:
// validate data before rendering and keep template actions infallible once they
// start emitting output or nested UI.
type Template struct {
	OuterHTMLTag string // Optional wrapper tag for partial templates, for example "div" or "tr"; empty renders unwrapped.
	Name         string // Template name to be looked up using Jaws.LookupTemplate.
	Dot          any    // Dot value to place in With.
}

var (
	_ jaws.UI                 = Template{} // statically ensure interface is defined
	_ jaws.ClickHandler       = Template{} // statically ensure interface is defined
	_ jaws.ContextMenuHandler = Template{} // statically ensure interface is defined
	_ jaws.InputHandler       = Template{} // statically ensure interface is defined
)

// templateState is the per-Element state a Template claims while rendering, held in the
// Element's widget state slot so the Template itself stays a stateless comparable value.
type templateState struct {
	// mu serializes the owned operations, which run on the rendering goroutine and
	// on the request loop goroutine during updates (mirrors ContainerHelper).
	mu sync.Mutex
	// owned are the Elements created while the template executed, in creation order.
	owned []*jaws.Element
}

// templateStateOf returns the state claimed for elem, or nil if no Template rendered it.
func templateStateOf(elem *jaws.Element) (st *templateState) {
	st, _ = jaws.ElementState(elem).(*templateState)
	return
}

// ownElement records child as created while the template executed. It is the
// [RequestWriter] element-created hook installed by execute, so it is called as soon
// as the Element exists and makes no assumption about whether it rendered.
func (st *templateState) ownElement(child *jaws.Element) {
	st.mu.Lock()
	st.owned = append(st.owned, child)
	st.mu.Unlock()
}

// takeOwnedElements returns the Elements created by the most recent execution and
// clears the tracking state, transferring responsibility for unregistering them to
// the caller. It implements elementOwner.
func (st *templateState) takeOwnedElements() (owned []*jaws.Element) {
	st.mu.Lock()
	owned, st.owned = st.owned, nil
	st.mu.Unlock()
	return
}

// restoreOwnedElements makes owned the tracked set again, for an update whose
// output was discarded and whose previous Elements still match the browser DOM.
func (st *templateState) restoreOwnedElements(owned []*jaws.Element) {
	st.mu.Lock()
	st.owned = owned
	st.mu.Unlock()
}

// String returns a debug representation of t.
func (tmpl Template) String() string {
	return fmt.Sprintf("{%q, %q, %s}", tmpl.OuterHTMLTag, tmpl.Name, tag.TagString(tmpl.Dot))
}

func (tmpl Template) lookup(elem *jaws.Element) (lookedUp *template.Template, err error) {
	err = errMissingTemplate(tmpl.Name)
	if lookedUp = elem.Request.Jaws.LookupTemplate(tmpl.Name); lookedUp != nil {
		err = nil
	}
	return
}

func (tmpl Template) auth(elem *jaws.Element) (auth jaws.Auth) {
	if f := elem.Request.Jaws.MakeAuth; f != nil {
		auth = f(elem.Request)
	} else {
		// Reuse the instance's shared DefaultAuth so its one-time fail-open warning
		// is logged once per Jaws, not once per render.
		auth = elem.Request.Jaws.DefaultAuth()
	}
	return
}

// execute runs the template with st owning the Elements it creates.
//
// st is a parameter rather than something execute loads, so every entry point has to
// establish the state deliberately — pageTemplate renders without going through render,
// and would otherwise silently track nothing.
func (tmpl Template) execute(elem *jaws.Element, w io.Writer, lookedUp *template.Template, st *templateState) (err error) {
	// The hook makes st own the Elements the template creates through this writer.
	// A nested RequestWriter.Template builds its own writer in its own execute, so
	// each level owns only its direct children and deeper ones are reached through
	// them; see appendOwnedElements.
	//
	// Tracking assumes html/template's synchronous execution on this goroutine. A
	// template action that renders from another goroutine is already unsupported: it
	// races on the shared io.Writer, and on the render as a whole.
	err = lookedUp.Execute(w, With{
		Element:       elem,
		RequestWriter: RequestWriter{Request: elem.Request, Writer: w, elementCreated: st.ownElement},
		Dot:           tmpl.Dot,
		Auth:          tmpl.auth(elem),
	})
	return
}

func writeTemplateWrapperStart(elem *jaws.Element, w io.Writer, outerHTMLTag string, attrs []string) (err error) {
	b := elem.Jid().AppendStartTagAttr(nil, outerHTMLTag)
	for _, attr := range attrs {
		if attr != "" {
			b = append(b, ' ')
			b = append(b, attr...)
		}
	}
	b = append(b, '>')
	_, err = w.Write(b)
	return
}

func (tmpl Template) render(elem *jaws.Element, w io.Writer, params []any) (err error) {
	// Claim the state slot before anything observable happens: tag registration, handler
	// registration and every write come after, so a contended Element fails having
	// changed nothing rather than having half-registered itself.
	st := &templateState{}
	if err = jaws.SetElementState(elem, st); err != nil {
		return
	}
	doWrap := tmpl.OuterHTMLTag != ""
	var expandedTags []any
	if expandedTags, err = tag.TagExpand(elem.Request, tmpl.Dot); err == nil {
		elem.Request.TagExpanded(elem, expandedTags)
		tags, handlers, attrs := jaws.ParseParams(params)
		elem.Tag(tags...)
		elem.AddHandlers(handlers...)
		var lookedUp *template.Template
		if lookedUp, err = tmpl.lookup(elem); err == nil {
			if doWrap {
				err = writeTemplateWrapperStart(elem, w, tmpl.OuterHTMLTag, attrs)
			}
			if err == nil {
				err = tmpl.execute(elem, w, lookedUp, st)
				if doWrap {
					// Always emit the closing tag, even when execute failed, to balance
					// the start tag already written above (mirrors
					// ContainerHelper.RenderContainer). The original execute error is
					// preserved; the close-write error is adopted only when err is nil.
					if _, werr := io.WriteString(w, "</"+tmpl.OuterHTMLTag+">"); err == nil {
						err = werr
					}
				}
				if err != nil {
					// The Element itself is unregistered by whoever created it
					// (RequestWriter.NewUI, or a container). Its nested UI is ours to
					// drop, since it will never be updated. The claim itself stays, so a
					// caller that handles this error can still update the Element.
					deleteOwnedElements(elem.Request, st.takeOwnedElements())
				}
			}
		}
	}
	return
}

// JawsRender renders t through the request's configured template lookupers,
// streaming output directly to w. Template execution has the best-effort error
// behavior described on [Template].
//
// It claims the [jaws.Element]'s widget state slot before doing anything else, so
// rendering a second Template into one Element fails with
// [jaws.ErrElementStateClaimed] having changed nothing.
func (tmpl Template) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	err = tmpl.render(elem, w, params)
	return
}

// JawsUpdate re-renders t into the template wrapper.
//
// Unwrapped templates have no generated DOM element to update, so updates are
// ignored; nested JaWS UI rendered by the template can still update through its own
// elements. The wrapper's SetInner is queued only after execution succeeds (see the
// best-effort error behavior on [Template]).
//
// A successful update unregisters the tracked Elements from the previous execution,
// along with any they own: SetInner replaces the DOM that held them. If execution
// fails nothing is queued, so the tracked Elements it created are unregistered
// instead and the previous ones stay live to match the unchanged DOM. See [Template]
// for which Elements are tracked.
//
// A wrapped Template updates only an Element some Template claimed while rendering it
// (see [jaws.SetElementState]); with no claim there is nothing to reconcile against, so
// it executes nothing and reports [ErrElementStateUnclaimed]. That makes a wrapped
// Template unusable as a [RequestWriter.Register] updater, since Register never renders
// its Element.
//
// Lookup, missing-state or execution errors are reported through
// [jaws.Request.MustLog], which may panic when no [jaws.Jaws.Logger] is configured.
func (tmpl Template) JawsUpdate(elem *jaws.Element) {
	if tmpl.OuterHTMLTag != "" {
		lookedUp, err := tmpl.lookup(elem)
		if err == nil {
			if st := templateStateOf(elem); st == nil {
				err = errElementStateUnclaimed(tmpl.Name)
			} else {
				// Detach before executing so the new Elements accumulate on their own.
				previous := st.takeOwnedElements()
				var sb strings.Builder
				if err = tmpl.execute(elem, &sb, lookedUp, st); err == nil {
					elem.SetInner(template.HTML(sb.String())) // #nosec G203
					deleteOwnedElements(elem.Request, previous)
				} else {
					deleteOwnedElements(elem.Request, st.takeOwnedElements())
					st.restoreOwnedElements(previous)
				}
			}
		}
		elem.Request.MustLog(err)
	}
}

// JawsClick delegates click events to t.Dot when it implements [jaws.ClickHandler].
func (tmpl Template) JawsClick(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	if h, ok := tmpl.Dot.(jaws.ClickHandler); ok {
		err = h.JawsClick(elem, click)
	}
	return
}

// JawsContextMenu delegates context-menu events to t.Dot when it implements
// [jaws.ContextMenuHandler].
func (tmpl Template) JawsContextMenu(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	if h, ok := tmpl.Dot.(jaws.ContextMenuHandler); ok {
		err = h.JawsContextMenu(elem, click)
	}
	return
}

// JawsInput delegates input events to t.Dot when it implements [jaws.InputHandler].
func (tmpl Template) JawsInput(elem *jaws.Element, value string) (err error) {
	err = jaws.ErrEventUnhandled
	if h, ok := tmpl.Dot.(jaws.InputHandler); ok {
		err = h.JawsInput(elem, value)
	}
	return
}

// NewTemplate returns a [Template] for rendering the named partial template with dot
// exposed as [With.Dot].
//
// outerHTMLTag names the generated wrapper element that owns the JaWS ID and
// render-time HTML attributes, or renders unwrapped (and [Template.JawsUpdate] has no
// wrapper to update) if empty. The name is resolved at render or update time via
// [jaws.Jaws.LookupTemplate]. See [Template] for the field semantics, event
// delegation and best-effort error behavior.
//
// The returned Template holds no per-Element state, so equal Templates are
// interchangeable and a container that rebuilds its children on every
// [jaws.Container.JawsContains] call still reuses their Elements. dot must be comparable
// at runtime and equal to itself. Use the returned Template as a value; taking its
// address is unsupported. See [Template].
func NewTemplate(outerHTMLTag, name string, dot any) Template {
	return Template{OuterHTMLTag: outerHTMLTag, Name: name, Dot: dot}
}

// Template renders the named partial template with dot exposed as [With.Dot],
// wrapping the output in a generated outerHTMLTag element (unwrapped if empty) that
// owns the JaWS ID and any HTML attrs in params. See [NewTemplate] and [Template].
func (rw RequestWriter) Template(outerHTMLTag, name string, dot any, params ...any) error {
	return rw.NewUI(NewTemplate(outerHTMLTag, name, dot), params...)
}
