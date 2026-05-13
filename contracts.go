package jaws

import (
	"html/template"
	"io"
)

// Container is implemented by UI values that render a dynamic list of child
// [UI] values.
type Container interface {
	// JawsContains must return a slice of hashable [UI] objects. The slice
	// contents must not be modified after returning it.
	JawsContains(elem *Element) (contents []UI)
}

// InitHandler allows initializing UI getters and setters before their use.
//
// You can of course initialize them in the call from the template engine,
// but at that point you don't have access to the [Element], [Element.Context]
// or [Element.Session].
type InitHandler interface {
	JawsInit(elem *Element) (err error)
}

// Logger matches the log/slog.Logger interface.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// Renderer renders the initial HTML for a [UI] object.
type Renderer interface {
	// JawsRender is called once per [Element] when rendering the initial webpage.
	// Do not call this yourself unless it is from within another JawsRender implementation.
	JawsRender(elem *Element, w io.Writer, params []any) error
}

// TemplateLookuper resolves a name to a *template.Template.
type TemplateLookuper interface {
	Lookup(name string) *template.Template
}

// UI defines the required methods on JaWS UI objects.
// In addition, all UI objects must be comparable so they can be used as map keys.
type UI interface {
	Renderer
	Updater
}

// Updater updates browser-side DOM for a dirty [Element].
type Updater interface {
	// JawsUpdate is called for an [Element] that has been marked dirty to update its HTML.
	// Do not call this yourself unless it is from within another JawsUpdate implementation.
	JawsUpdate(elem *Element)
}

// ClickHandler handles click events sent from the browser.
type ClickHandler interface {
	// JawsClick is called when an [Element]'s HTML element or something within it
	// is clicked in the browser.
	//
	// [Click.Name] is taken from the first name HTML attribute or HTML
	// 'button' textContent found when traversing the DOM. It may be empty.
	JawsClick(elem *Element, click Click) (err error)
}

// ContextMenuHandler handles context-menu events sent from the browser.
type ContextMenuHandler interface {
	// JawsContextMenu is called when an [Element]'s HTML element or something
	// within it receives a context menu event in the browser.
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
	Data() map[string]any // returns authenticated user data, or nil
	Email() string        // returns authenticated user email, or an empty string
	IsAdmin() bool        // return true if admins are defined and current user is one, or if no admins are defined
}

// MakeAuthFn constructs an [Auth] value for a [Request].
type MakeAuthFn func(rq *Request) Auth

// DefaultAuth is the permissive default [Auth] implementation.
type DefaultAuth struct{}

func (DefaultAuth) Data() map[string]any { return nil }
func (DefaultAuth) Email() string        { return "" }
func (DefaultAuth) IsAdmin() bool        { return true }
