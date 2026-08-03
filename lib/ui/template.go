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
// A Template tracks the [jaws.Element] values created while its template executes,
// so it must back at most one live Element. Construct a distinct Template for each
// place the template is rendered; [RequestWriter.Template] does so for every call.
// Its Dot and any callbacks reached during execution are shared with anything else
// referencing them and must be safe for those calls.
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
// The Elements a template creates through [RequestWriter.NewUI] — the path taken by
// every RequestWriter widget helper except [RequestWriter.Register] and
// [RequestWriter.RadioGroup], including a nested [RequestWriter.Template] — belong to
// the Template that rendered them: when [Template.JawsUpdate] replaces the wrapper's
// content, those Elements are unregistered along with the DOM that held them, and a
// nested widget's own Elements go with it.
//
// Those two helpers create their Elements through [jaws.Request.NewElement] instead,
// so a Template neither tracks nor unregisters them. Their cleanup is left to the
// browser, which reports the JaWS ids it removed from the DOM as the wrapper's new
// content is applied, and the [jaws.Request] unregisters those Elements. An Element
// whose id never reaches the DOM has nothing to report it and stays registered until
// the Request ends: one from an execution that failed before its markup was
// delivered, or from a Register call whose returned Jid the template discards.
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
	// mu serializes the owned operations, which run on the rendering goroutine and
	// on the request loop goroutine during updates (mirrors ContainerHelper).
	mu sync.Mutex
	// owned are the Elements created while the template executed, in creation
	// order. It tracks one live Element's nested UI.
	owned []*jaws.Element
}

var (
	_ jaws.UI                 = (*Template)(nil) // statically ensure interface is defined
	_ jaws.ClickHandler       = (*Template)(nil) // statically ensure interface is defined
	_ jaws.ContextMenuHandler = (*Template)(nil) // statically ensure interface is defined
	_ jaws.InputHandler       = (*Template)(nil) // statically ensure interface is defined
)

// The methods below dereference tmpl without a nil check, like the other
// pointer-based widgets in this package (*Span, *Container, *JsVar): jaws.UI accepts
// a typed nil and dispatches to it, but leaves surviving a nil receiver to the
// concrete type, and this package documents that none of its widgets do (see doc.go).
// A zero &Template{} is the supported empty value and reports ErrMissingTemplate.

// String returns a debug representation of t.
func (tmpl *Template) String() string {
	return fmt.Sprintf("{%q, %q, %s}", tmpl.OuterHTMLTag, tmpl.Name, tag.TagString(tmpl.Dot))
}

// ownElement records child as created while tmpl's template executed. It is the
// [RequestWriter] element-rendered hook installed by execute.
func (tmpl *Template) ownElement(child *jaws.Element) (err error) {
	tmpl.mu.Lock()
	tmpl.owned = append(tmpl.owned, child)
	tmpl.mu.Unlock()
	return
}

// takeOwnedElements returns the Elements created by the most recent execution and
// clears the tracking state, transferring responsibility for unregistering them to
// the caller. It implements elementOwner.
func (tmpl *Template) takeOwnedElements() (owned []*jaws.Element) {
	tmpl.mu.Lock()
	owned, tmpl.owned = tmpl.owned, nil
	tmpl.mu.Unlock()
	return
}

// restoreOwnedElements makes owned the tracked set again, for an update whose
// output was discarded and whose previous Elements still match the browser DOM.
func (tmpl *Template) restoreOwnedElements(owned []*jaws.Element) {
	tmpl.mu.Lock()
	tmpl.owned = owned
	tmpl.mu.Unlock()
}

func (tmpl *Template) lookup(elem *jaws.Element) (lookedUp *template.Template, err error) {
	err = errMissingTemplate(tmpl.Name)
	if lookedUp = elem.Request.Jaws.LookupTemplate(tmpl.Name); lookedUp != nil {
		err = nil
	}
	return
}

func (tmpl *Template) auth(elem *jaws.Element) (auth jaws.Auth) {
	if f := elem.Request.Jaws.MakeAuth; f != nil {
		auth = f(elem.Request)
	} else {
		// Reuse the instance's shared DefaultAuth so its one-time fail-open warning
		// is logged once per Jaws, not once per render.
		auth = elem.Request.Jaws.DefaultAuth()
	}
	return
}

func (tmpl *Template) execute(elem *jaws.Element, w io.Writer, lookedUp *template.Template) (err error) {
	// The hook makes tmpl own the Elements the template creates through this writer.
	// A nested RequestWriter.Template builds its own writer in its own execute, so
	// each level owns only its direct children and deeper ones are reached through
	// them; see appendOwnedElements.
	//
	// Tracking assumes html/template's synchronous execution on this goroutine. A
	// template action that renders from another goroutine is already unsupported: it
	// races on the shared io.Writer, and on the render as a whole.
	err = lookedUp.Execute(w, With{
		Element:       elem,
		RequestWriter: RequestWriter{Request: elem.Request, Writer: w, elementRendered: tmpl.ownElement},
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

func (tmpl *Template) render(elem *jaws.Element, w io.Writer, params []any) (err error) {
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
				err = tmpl.execute(elem, w, lookedUp)
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
					// drop, since it will never be updated.
					deleteOwnedElements(elem.Request, tmpl.takeOwnedElements())
				}
			}
		}
	}
	return
}

// JawsRender renders t through the request's configured template lookupers,
// streaming output directly to w. Template execution has the best-effort error
// behavior described on [Template].
func (tmpl *Template) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
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
// Lookup or execution errors are reported through [jaws.Request.MustLog], which may
// panic when no [jaws.Jaws.Logger] is configured.
func (tmpl *Template) JawsUpdate(elem *jaws.Element) {
	if tmpl.OuterHTMLTag != "" {
		lookedUp, err := tmpl.lookup(elem)
		if err == nil {
			// Detach before executing so the new Elements accumulate on their own; a
			// lookup failure returns above with the set untouched.
			previous := tmpl.takeOwnedElements()
			var sb strings.Builder
			if err = tmpl.execute(elem, &sb, lookedUp); err == nil {
				elem.SetInner(template.HTML(sb.String())) // #nosec G203
				deleteOwnedElements(elem.Request, previous)
			} else {
				deleteOwnedElements(elem.Request, tmpl.takeOwnedElements())
				tmpl.restoreOwnedElements(previous)
			}
		}
		elem.Request.MustLog(err)
	}
}

// JawsClick delegates click events to t.Dot when it implements [jaws.ClickHandler].
func (tmpl *Template) JawsClick(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	if h, ok := tmpl.Dot.(jaws.ClickHandler); ok {
		err = h.JawsClick(elem, click)
	}
	return
}

// JawsContextMenu delegates context-menu events to t.Dot when it implements
// [jaws.ContextMenuHandler].
func (tmpl *Template) JawsContextMenu(elem *jaws.Element, click jaws.Click) (err error) {
	err = jaws.ErrEventUnhandled
	if h, ok := tmpl.Dot.(jaws.ContextMenuHandler); ok {
		err = h.JawsContextMenu(elem, click)
	}
	return
}

// JawsInput delegates input events to t.Dot when it implements [jaws.InputHandler].
func (tmpl *Template) JawsInput(elem *jaws.Element, value string) (err error) {
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
// The returned Template is request-scoped and tracks the Elements one live
// [jaws.Element] renders; call NewTemplate again for each place it is rendered.
func NewTemplate(outerHTMLTag, name string, dot any) *Template {
	return &Template{OuterHTMLTag: outerHTMLTag, Name: name, Dot: dot}
}

// Template renders the named partial template with dot exposed as [With.Dot],
// wrapping the output in a generated outerHTMLTag element (unwrapped if empty) that
// owns the JaWS ID and any HTML attrs in params. See [NewTemplate] and [Template].
func (rw RequestWriter) Template(outerHTMLTag, name string, dot any, params ...any) error {
	return rw.NewUI(NewTemplate(outerHTMLTag, name, dot), params...)
}
