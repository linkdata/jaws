package jaws

import (
	"html/template"
	"io"
	"sync"
)

// Container is implemented by UI values that render a dynamic list of child
// [UI] values.
type Container interface {
	// JawsContains returns the current child [UI] values contained by elem.
	//
	// The returned [UI] values must be comparable and equal to themselves, since they
	// are used as map keys (see [UI] for the requirement); a child that is a nil
	// interface, not comparable at runtime, or not equal to itself (such as one holding
	// NaN) cancels the [Request] instead of being reconciled. A typed nil is usable.
	// The slice contents must not be modified after returning it. Returning a usable
	// child UI again from a later call lets the container reuse its existing live
	// [Element]. Each child must render one direct DOM node carrying its Element's JaWS
	// ID, because reconciliation removes and orders that node. The same UI may occur more
	// than once in one returned slice only when its type documents support for backing
	// multiple live Elements. A child UI must not be shared with a different [Request].
	JawsContains(elem *Element) (contents []UI)
}

// Logger is satisfied by a [*log/slog.Logger] via its Info, Warn and Error methods.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Renderer renders the initial HTML for a [UI] object.
type Renderer interface {
	// JawsRender is called once per [Element] when rendering the initial webpage.
	// Do not call this yourself unless it is from within another JawsRender implementation.
	// The engine does not invoke this once the [Element] is deleted (see [Element.Deleted]).
	//
	// A delegating renderer and its delegates may claim the Element's widget state
	// slot only once. A later claim fails with [ErrElementStateClaimed]; see
	// [SetElementState].
	JawsRender(elem *Element, w io.Writer, params []any) error
}

// TemplateLookuper resolves a name to a *template.Template.
type TemplateLookuper interface {
	Lookup(name string) *template.Template
}

// UI defines the required methods on JaWS UI objects.
//
// A UI value is request-scoped. Once it has been used to create an [Element]
// for one [Request], it must not be used to create an Element for another
// Request. Construct a fresh UI value for each Request.
//
// Within its owning Request, a UI value must back at most one live Element
// unless its concrete type documents support for multiple live Elements. Such
// a type must not retain state on the shared UI value that can differ between
// those Elements — [SetElementState] gives it somewhere else to keep such state,
// keyed to the Element rather than to the widget, but opting in remains the concrete
// type's decision to document. To render the same application state more than once,
// construct distinct UI values that share getters, setters, handlers or tags.
// An Element stops being live when it is deleted or its owning Request lifecycle
// ends.
//
// Application state referenced by UI values may be shared across Requests when
// synchronized as required.
//
// In addition, all UI objects must be comparable and equal to themselves, so they
// can be used as map keys. The compile-time type must be comparable; the container
// widgets additionally check each child value at runtime and cancel the Request, in
// every build, when it is a nil interface, not comparable at runtime (for example a
// comparable struct holding a func in an interface field), or not equal to itself (a
// value holding NaN), with a cause matching
// [github.com/linkdata/jaws/lib/tag.ErrNotUsableAsTag] under errors.Is. That is the
// only place a raw UI value is used as a map key; outside a container
// [Request.NewElement] asserts runtime comparability in debug builds. Callers must
// ensure UI values are genuinely comparable and reflexive.
//
// A typed nil (a non-nil interface holding a nil pointer) meets those key
// requirements, being comparable and equal to itself, so JaWS accepts one and
// dispatches render, update and event calls to it like any other value; only a nil
// UI interface is treated as a no-op. Surviving those calls with a nil receiver is a
// property of the concrete type rather than a requirement of this contract: a type
// may document that it tolerates one, and a type that does not will panic when
// dereferencing its fields. Passing a nil pointer of such a type is therefore a
// caller error, not a framework-handled case. The pointer-valued widgets in
// [github.com/linkdata/jaws/lib/ui] dereference their fields, and no widget there
// documents nil-receiver tolerance.
type UI interface {
	Renderer
	Updater
}

// Updater updates browser-side DOM for a dirty [Element].
type Updater interface {
	// JawsUpdate is called for an [Element] that has been marked dirty to update its HTML.
	// Do not call this yourself unless it is from within another JawsUpdate implementation.
	// The engine does not invoke this once the [Element] is deleted (see [Element.Deleted]).
	// A UI implementation that delegates rendering and updating must delegate both calls
	// to the same UI widget. Rendering elem through one widget and updating it through
	// another is unsupported.
	JawsUpdate(elem *Element)
}

// ClickHandler handles click events sent from the browser.
type ClickHandler interface {
	// JawsClick is called for non-input-origin browser clicks.
	//
	// The client sends clicks from an [Element]'s HTML element and from
	// non-form-control descendants. Clicks whose event target is an input,
	// select, textarea or option element, or inside one, are left to native
	// input handling and do not invoke JawsClick on an ancestor.
	//
	// [Click.Name] is the first name HTML attribute or 'button' textContent
	// found while walking from the event target up through its ancestors. If none
	// is found it falls back to the event target's HTML id, so it is empty only
	// when the target has no id either.
	JawsClick(elem *Element, click Click) (err error)
}

// ContextMenuHandler handles context-menu events sent from the browser.
type ContextMenuHandler interface {
	// JawsContextMenu is called for non-input-origin browser context menus.
	//
	// The client sends context-menu events from an [Element]'s HTML element and
	// from non-form-control descendants. Events whose target is an input, select,
	// textarea or option element, or inside one, are left to native browser
	// handling and do not invoke JawsContextMenu on an ancestor.
	JawsContextMenu(elem *Element, click Click) (err error)
}

// InitialHTMLAttrHandler provides attributes for initial [Element] rendering.
type InitialHTMLAttrHandler interface {
	// JawsInitialHTMLAttr returns attributes for elem's initial render, or an empty string.
	//
	// Callers must not hold a lock protecting the handler or its source. The method
	// must synchronize shared state. Its result need not share a state snapshot with
	// other values read during rendering.
	JawsInitialHTMLAttr(elem *Element) (s template.HTMLAttr)
}

// Auth describes authentication data available to templates through ui.With.
type Auth interface {
	// Data returns authenticated user data, or nil.
	Data() map[string]any
	// Email returns the authenticated user email, or an empty string.
	Email() string
	// IsAdmin reports whether the authenticated user has administrator access.
	IsAdmin() bool
}

// MakeAuthFn constructs an [Auth] value for a [Request].
//
// Set [Jaws.MakeAuth] to your implementation to enforce real authorization. If
// [Jaws.MakeAuth] is left nil, templates receive [DefaultAuth], which is
// fail-open: see its documentation.
//
// It is a type alias so a bare func value can be assigned without conversion,
// matching the sibling callback types [ConnectFn], [InputFn] and [HandleFunc].
type MakeAuthFn = func(rq *Request) Auth

// DefaultAuth is the permissive default [Auth] implementation used for templates
// when [Jaws.MakeAuth] is nil.
//
// SECURITY: DefaultAuth.IsAdmin always returns true. Because it is substituted
// whenever [Jaws.MakeAuth] is unset, a template that gates privileged UI on
// {{if .Auth.IsAdmin}} will render that UI to EVERY visitor on any instance that
// forgot to set [Jaws.MakeAuth]. Data and Email are fail-safe (nil / empty); only
// IsAdmin is fail-open. Always set [Jaws.MakeAuth] in production, and treat a nil
// MakeAuth as "no authorization configured", not "deny".
type DefaultAuth struct {
	// once and logger are unexported so the type's public method set is exactly the
	// [Auth] interface, without promoting sync.Once.Do or the Logger methods onto this
	// security-sensitive type.
	once   sync.Once
	logger Logger
}

// Data returns no authenticated user data.
func (*DefaultAuth) Data() map[string]any { return nil }

// Email returns an empty authenticated user email.
func (*DefaultAuth) Email() string { return "" }

// IsAdmin returns true for every caller.
//
// If configured with a logger, it logs one warning that [Jaws.MakeAuth] is unset
// and authorization is fail-open.
func (da *DefaultAuth) IsAdmin() bool {
	// Warn loudly about the fail-open authorization default. When MakeAuth is nil
	// templates receive DefaultAuth, IsAdmin() returns true for everyone, so
	// any {{if .Auth.IsAdmin}}-gated UI is shown to all visitors.
	var logger Logger
	da.once.Do(func() {
		logger = da.logger
	})
	// Logger.Warn may re-enter IsAdmin, so invoke it after once.Do returns.
	if logger != nil {
		logger.Warn("jaws: no MakeAuth; DefaultAuth.IsAdmin returns true")
	}
	return true
}

// DefaultAuth returns the shared fail-open [DefaultAuth] used for templates when
// [Jaws.MakeAuth] is nil.
//
// The returned value logs at most one warning through the [Jaws.Logger] in
// effect when DefaultAuth is first called.
func (jw *Jaws) DefaultAuth() *DefaultAuth {
	jw.defaultAuthOnce.Do(func() {
		jw.defaultAuthVal = &DefaultAuth{logger: jw.Logger}
	})
	return jw.defaultAuthVal
}
