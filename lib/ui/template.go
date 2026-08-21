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

// Template renders a named Go [html/template] partial through JaWS.
//
// Use Templates as values; [NewTemplate] is the usual constructor for wrapped
// Templates. A Template may back multiple live [jaws.Element] values in one
// [jaws.Request]; its Dot and callbacks must support all of their render, update,
// and event calls. Taking a Template's address is unsupported because it changes
// container reuse from value identity to pointer identity.
//
// Dot may be a nil interface. Otherwise the Template must be comparable at
// runtime and equal to itself, and Dot must be usable as a tag under
// [tag.TagExpand].
//
// If Dot implements [jaws.InitialHTMLAttrHandler], its callback supplies wrapper
// attributes separately for each wrapped Element's initial render. An unwrapped
// Template has no attribute target and does not invoke the callback. The callback
// is not invoked during [Template.JawsUpdate].
//
// OuterHTMLTag names the wrapper that receives the JaWS ID and render-time HTML
// attributes from render parameters and Dot. An empty field renders without a
// wrapper, making [Template.JawsUpdate] a no-op. [NewTemplate] defaults an empty
// wrapper argument to "div". The named template must be a partial; use [Handler]
// for a complete document.
//
// A Template owns the Elements created through its [RequestWriter]. A successful
// update unregisters Elements from the previous execution. Elements created by a
// failed execution are also unregistered.
//
// Replacing wrapper contents reports all managed descendants being removed in
// one WebSocket message, subject to [jaws.Request.ServeHTTP]'s 32 KiB inbound
// limit. Split large trees into independently updated nested wrappers.
//
// Execution is not transactional. An error may leave partial output, queued
// messages, or application side effects in place.
type Template struct {
	OuterHTMLTag string // Wrapper element; empty renders unwrapped and disables JawsUpdate.
	Name         string // Template name to be looked up using Jaws.LookupTemplate.
	Dot          any    // Template data, tag source, event delegate, and initial-attribute source.
}

var (
	_ jaws.UI                 = Template{} // statically ensure interface is defined
	_ jaws.ClickHandler       = Template{} // statically ensure interface is defined
	_ jaws.ContextMenuHandler = Template{} // statically ensure interface is defined
	_ jaws.InputHandler       = Template{} // statically ensure interface is defined
)

// templateState is the per-Element state a Template claims while rendering.
type templateState struct {
	// mu serializes the owned operations, which run on the rendering goroutine and
	// on the request loop goroutine during updates (mirrors containerState).
	mu sync.Mutex
	// owned are the Elements created while the template executed, in creation order.
	// Keeping ownership here leaves Template as a stateless comparable value.
	owned []*jaws.Element
}

// templateStateOf returns the state claimed for elem, or nil if no Template rendered it.
func templateStateOf(elem *jaws.Element) (st *templateState) {
	st, _ = jaws.ElementState(elem).(*templateState)
	return
}

// ownElement records child as created while the template executed.
func (st *templateState) ownElement(child *jaws.Element) {
	// execute installs this as RequestWriter's creation hook. Record the Element
	// immediately without assuming that it rendered.
	st.mu.Lock()
	st.owned = append(st.owned, child)
	st.mu.Unlock()
}

// takeOwnedElements returns the Elements created by the most recent execution and
// clears the tracking state, transferring responsibility for unregistering them to
// the caller.
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
	// Claim the state slot before anything observable happens: application callbacks,
	// tag and handler registration, and every write come after, so a contended Element
	// fails having changed nothing rather than having half-registered itself.
	st := &templateState{}
	if err = jaws.SetElementState(elem, st); err != nil {
		return
	}
	doWrap := tmpl.OuterHTMLTag != ""
	var expandedTags []any
	if expandedTags, err = tag.TagExpand(tmpl.Dot); err == nil {
		elem.Request.TagExpanded(elem, expandedTags)
		tags, handlers, attrs := jaws.ParseParams(params)
		elem.Tag(tags...)
		elem.AddHandlers(handlers...)
		var lookedUp *template.Template
		if lookedUp, err = tmpl.lookup(elem); err == nil {
			if doWrap {
				for _, attr := range elem.ApplyInitialHTMLAttr(tmpl.Dot) {
					attrs = append(attrs, string(attr))
				}
				err = writeTemplateWrapperStart(elem, w, tmpl.OuterHTMLTag, attrs)
			}
			if err == nil {
				err = tmpl.execute(elem, w, lookedUp, st)
				if doWrap {
					// Always emit the closing tag, even when execute failed, to balance
					// the start tag already written above (mirrors
					// Container.render). The original execute error is
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

// JawsRender renders t through the request's configured template lookupers.
//
// If elem's widget state is occupied, JawsRender returns
// [jaws.ErrElementStateClaimed] without output. Other errors may leave partial
// output or side effects as described on [Template].
func (tmpl Template) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	err = tmpl.render(elem, w, params)
	return
}

// JawsUpdate re-renders the Template into its wrapper.
//
// An empty OuterHTMLTag has no DOM target, so the update is a no-op. Otherwise
// elem must have been rendered by an equal Template value; using an unequal value
// is unsupported. After a successful lookup, missing Template state reports
// [ErrElementStateUnclaimed].
//
// On success, JawsUpdate replaces the wrapper content and unregisters Elements
// from the previous execution. On execution failure, it keeps the previous DOM and
// Elements and unregisters Elements created by the failed attempt.
//
// Lookup, state, and execution errors are reported through
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
// render-time HTML attributes. If outerHTMLTag is empty, "div" is used. Choose a tag
// suitable for the DOM context. For an unwrapped fragment, use html/template's native
// {{template "name" pipeline}} action. The name is resolved at render and update time.
//
// dot may be a nil interface. Otherwise it must make the returned Template
// comparable and equal to itself, and it must be usable as a tag under
// [tag.TagExpand]. Use the returned Template as a value; taking its address is
// unsupported. If dot implements [jaws.InitialHTMLAttrHandler], its callback
// supplies attributes separately for each generated wrapper's initial render.
func NewTemplate(outerHTMLTag, name string, dot any) Template {
	if outerHTMLTag == "" {
		outerHTMLTag = "div"
	}
	return newTemplate(outerHTMLTag, name, dot)
}

func newTemplate(outerHTMLTag, name string, dot any) Template {
	return Template{OuterHTMLTag: outerHTMLTag, Name: name, Dot: dot}
}

// Template renders the named partial template with dot exposed as [With.Dot].
//
// The generated outerHTMLTag wrapper owns the JaWS ID, HTML attributes in params,
// and attributes returned by dot when it implements
// [jaws.InitialHTMLAttrHandler]. An empty outerHTMLTag defaults to "div". See
// [NewTemplate].
func (rw RequestWriter) Template(outerHTMLTag, name string, dot any, params ...any) error {
	return rw.NewUI(NewTemplate(outerHTMLTag, name, dot), params...)
}
