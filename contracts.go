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
	// The returned [UI] values must be comparable, since they are used as map keys
	// (see [UI] for the comparability requirement), and the slice contents must not
	// be modified after returning it.
	JawsContains(elem *Element) (contents []UI)
}

// InitHandler allows initializing UI getters and setters before their use.
//
// You can of course initialize them in the call from the template engine,
// but at that point you don't have access to the [Element], [Request.Context]
// or [Request.Session].
type InitHandler interface {
	JawsInit(elem *Element) (err error)
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
	JawsRender(elem *Element, w io.Writer, params []any) error
}

// TemplateLookuper resolves a name to a *template.Template.
type TemplateLookuper interface {
	Lookup(name string) *template.Template
}

// UI defines the required methods on JaWS UI objects.
//
// In addition, all UI objects must be comparable so they can be used as map keys.
// The compile-time type must be comparable; debug builds additionally perform a
// runtime value-level check in [Request.NewElement] and panic on a value that is
// statically comparable but not comparable at runtime (for example a comparable
// struct holding a func in an interface field). Production builds rely on the
// static check alone, so such a value is accepted and instead panics when first
// used as a map key; callers must therefore ensure UI values are genuinely
// comparable.
type UI interface {
	Renderer
	Updater
}

// Updater updates browser-side DOM for a dirty [Element].
type Updater interface {
	// JawsUpdate is called for an [Element] that has been marked dirty to update its HTML.
	// Do not call this yourself unless it is from within another JawsUpdate implementation.
	// The engine does not invoke this once the [Element] is deleted (see [Element.Deleted]).
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

// InitialHTMLAttrHandler can add attributes during initial [Element] rendering.
type InitialHTMLAttrHandler interface {
	// JawsInitialHTMLAttr is called when an [Element] is initially rendered,
	// and may return an initial HTML attribute string to write out.
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
//
// The once and logger fields are unexported so the type's public method set is
// exactly the [Auth] interface; it does not promote sync.Once.Do or the Logger
// methods onto this security-sensitive type.
type DefaultAuth struct {
	once   sync.Once
	logger Logger
}

// Data returns no authenticated user data.
func (*DefaultAuth) Data() map[string]any { return nil }

// Email returns an empty authenticated user email.
func (*DefaultAuth) Email() string { return "" }

// IsAdmin returns true for every caller.
//
// If a logger was supplied at construction, it logs a one-time warning that
// [Jaws.MakeAuth] is unset and authorization is fail-open.
func (da *DefaultAuth) IsAdmin() bool {
	// Warn loudly about the fail-open authorization default. When MakeAuth is nil
	// templates receive DefaultAuth, IsAdmin() returns true for everyone, so
	// any {{if .Auth.IsAdmin}}-gated UI is shown to all visitors.
	da.once.Do(func() {
		if da.logger != nil {
			da.logger.Warn("jaws: no MakeAuth; DefaultAuth.IsAdmin returns true")
		}
	})
	return true
}

// DefaultAuth returns the shared fail-open [DefaultAuth] used for templates when
// [Jaws.MakeAuth] is nil.
//
// It is created on first use and reused, so the [sync.Once] warning in
// [DefaultAuth.IsAdmin] fires at most once per [Jaws] rather than once per
// template render. The value of [Jaws.Logger] in effect at first use is captured.
func (jw *Jaws) DefaultAuth() *DefaultAuth {
	jw.defaultAuthOnce.Do(func() {
		jw.defaultAuthVal = &DefaultAuth{logger: jw.Logger}
	})
	return jw.defaultAuthVal
}
