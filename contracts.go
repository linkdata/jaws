package jaws

import (
	"html/template"
	"io"

	"github.com/linkdata/jaws/lib/what"
)

type Container interface {
	// JawsContains must return a slice of hashable UI objects. The slice contents must not be modified after returning it.
	JawsContains(e *Element) (contents []UI)
}

// InitHandler allows initializing UI getters and setters before their use.
//
// You can of course initialize them in the call from the template engine,
// but at that point you don't have access to the Element, Element.Context
// or Element.Session.
type InitHandler interface {
	JawsInit(e *Element) (err error)
}

// Logger matches the log/slog.Logger interface.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type Renderer interface {
	// JawsRender is called once per Element when rendering the initial webpage.
	// Do not call this yourself unless it's from within another JawsRender implementation.
	JawsRender(e *Element, w io.Writer, params []any) error
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

type Updater interface {
	// JawsUpdate is called for an Element that has been marked dirty to update it's HTML.
	// Do not call this yourself unless it's from within another JawsUpdate implementation.
	JawsUpdate(e *Element)
}

type ClickHandler interface {
	// JawsClick is called when an Element's HTML element or something within it
	// is clicked in the browser.
	//
	// Click.Name is taken from the first 'name' HTML attribute or HTML
	// 'button' textContent found when traversing the DOM. It may be empty.
	JawsClick(e *Element, click Click) (err error)
}

type ContextMenuHandler interface {
	// JawsContextMenu is called when an Element's HTML element or something
	// within it receives a context menu event in the browser.
	JawsContextMenu(e *Element, click Click) (err error)
}

type clickHandlerWrapper struct{ ClickHandler }

func (chw clickHandlerWrapper) JawsEvent(*Element, what.What, string) error {
	return ErrEventUnhandled
}

type contextMenuHandlerWrapper struct{ ContextMenuHandler }

func (cmw contextMenuHandlerWrapper) JawsEvent(*Element, what.What, string) error {
	return ErrEventUnhandled
}

type Auth interface {
	Data() map[string]any // returns authenticated user data, or nil
	Email() string        // returns authenticated user email, or an empty string
	IsAdmin() bool        // return true if admins are defined and current user is one, or if no admins are defined
}

type MakeAuthFn func(*Request) Auth

type DefaultAuth struct{}

func (DefaultAuth) Data() map[string]any { return nil }
func (DefaultAuth) Email() string        { return "" }
func (DefaultAuth) IsAdmin() bool        { return true }
