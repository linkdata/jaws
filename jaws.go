package jaws

import (
	"sync"
	"time"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/assets"
	"github.com/linkdata/jaws/core/tags"
	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/ui"
)

// The point of this is to not have a zillion files in the repository root
// while keeping the import path unchanged.
//
// Most exports use direct assignment to avoid wrapper overhead.
// Generic functions must be wrapped since they cannot be assigned without instantiation.

type (
	// Jid is the identifier type used for HTML elements managed by JaWS.
	//
	// It is provided as a convenience alias to the value defined in the jid
	// subpackage so applications do not have to import that package directly
	// when working with element IDs.
	Jid = jid.Jid
	// Jaws holds the server-side state and configuration for a JaWS instance.
	//
	// A single Jaws value coordinates template lookup, session handling and the
	// request lifecycle that keeps the browser and backend synchronized via
	// WebSockets. The zero value is not ready for use; construct instances with
	// New to ensure the helper goroutines and static assets are prepared.
	Jaws = core.Jaws
	// Request maintains the state for a JaWS WebSocket connection, and handles processing
	// of events and broadcasts.
	//
	// Note that we have to store the context inside the struct because there is no call chain
	// between the Request being created and it being used once the WebSocket is created.
	Request = core.Request
	// An Element is an instance of a *Request, an UI object and a Jid.
	Element = core.Element
	// UI defines the required methods on JaWS UI objects.
	// In addition, all UI objects must be comparable so they can be used as map keys.
	UI       = core.UI
	Updater  = core.Updater
	Renderer = core.Renderer
	// TemplateLookuper resolves a name to a *template.Template.
	TemplateLookuper = core.TemplateLookuper
	// HandleFunc matches the signature of http.ServeMux.Handle().
	HandleFunc = core.HandleFunc
	Formatter  = core.Formatter
	Auth       = core.Auth
	// InitHandler allows initializing UI getters and setters before their use.
	//
	// You can of course initialize them in the call from the template engine,
	// but at that point you don't have access to the Element, Element.Context
	// or Element.Session.
	InitHandler          = core.InitHandler
	ClickHandler         = core.ClickHandler
	EventHandler         = core.EventHandler
	SelectHandler        = core.SelectHandler
	Container            = core.Container
	Getter[T comparable] = core.Getter[T]
	Setter[T comparable] = core.Setter[T]
	Binder[T comparable] = core.Binder[T]
	// A HTMLGetter is the primary way to deliver generated HTML content to dynamic HTML nodes.
	HTMLGetter = core.HTMLGetter
	// Logger matches the log/slog.Logger interface.
	Logger    = core.Logger
	RWLocker  = core.RWLocker
	TagGetter = tags.TagGetter
	// NamedBool stores a named boolen value with a HTML representation.
	NamedBool = core.NamedBool
	// NamedBoolArray stores the data required to support HTML 'select' elements
	// and sets of HTML radio buttons. It it safe to use from multiple goroutines
	// concurrently.
	NamedBoolArray = core.NamedBoolArray
	Session        = core.Session
	Tag            = tags.Tag
	// TestRequest is a request harness intended for tests.
	//
	// Exposed for testing only.
	TestRequest = core.TestRequest
)

var (
	ErrEventUnhandled        = core.ErrEventUnhandled
	ErrIllegalTagType        = tags.ErrIllegalTagType // ErrIllegalTagType is returned when a UI tag type is disallowed
	ErrNotComparable         = tags.ErrNotComparable
	ErrNotUsableAsTag        = tags.ErrNotUsableAsTag
	ErrNoWebSocketRequest    = core.ErrNoWebSocketRequest
	ErrPendingCancelled      = core.ErrPendingCancelled
	ErrValueUnchanged        = core.ErrValueUnchanged
	ErrValueNotSettable      = core.ErrValueNotSettable
	ErrRequestAlreadyClaimed = core.ErrRequestAlreadyClaimed
	ErrJavascriptDisabled    = core.ErrJavascriptDisabled
	ErrTooManyTags           = tags.ErrTooManyTags
)

const (
	// ISO8601 is the date format used by date input widgets (YYYY-MM-DD).
	ISO8601 = assets.ISO8601
)

var (
	// New allocates a JaWS instance with the default configuration.
	//
	// The returned Jaws value is ready for use: static assets are embedded,
	// internal goroutines are configured and the request pool is primed. Call
	// Close when the instance is no longer needed to free associated resources.
	New = core.New
	// JawsKeyString returns the string to be used for the given JaWS key.
	JawsKeyString = assets.JawsKeyString
	WriteHTMLTag  = core.WriteHTMLTag
	// HTMLGetterFunc wraps a function and returns a HTMLGetter.
	HTMLGetterFunc = core.HTMLGetterFunc
	// StringGetterFunc wraps a function and returns a Getter[string]
	StringGetterFunc = core.StringGetterFunc
	// MakeHTMLGetter returns a HTMLGetter for v.
	//
	// Depending on the type of v, we return:
	//
	//   - HTMLGetter: `JawsGetHTML(e *Element) template.HTML` to be used as-is.
	//   - Getter[string]: `JawsGet(elem *Element) string` that will be escaped using `html.EscapeString`.
	//   - Formatter: `Format("%v") string` that will be escaped using `html.EscapeString`.
	//   - fmt.Stringer: `String() string` that will be escaped using `html.EscapeString`.
	//   - a static `template.HTML` or `string` to be used as-is with no HTML escaping.
	//   - everything else is rendered using `fmt.Sprint()` and escaped using `html.EscapeString`.
	//
	// WARNING: Plain string values are NOT HTML-escaped. This is intentional so that
	// HTML markup can be passed conveniently from Go templates (e.g. `{{$.Span "<i>text</i>"}}`).
	// Never pass untrusted user input as a plain string; use [template.HTML] to signal
	// that the content is trusted, or wrap user input in a [Getter] or [fmt.Stringer]
	// so it will be escaped automatically.
	MakeHTMLGetter    = core.MakeHTMLGetter
	NewNamedBool      = core.NewNamedBool
	NewNamedBoolArray = core.NewNamedBoolArray
	// NewTestRequest creates a TestRequest for use when testing.
	// Passing nil for hr will create a "GET /" request with no body.
	//
	// Exposed for testing only.
	NewTestRequest = core.NewTestRequest
)

// Bind returns a Binder[T] with the given sync.Locker (or RWLocker) and a pointer to the underlying value of type T.
//
// The pointer will be used as the UI tag.
func Bind[T comparable](l sync.Locker, p *T) Binder[T] {
	return core.Bind(l, p)
}

/*
	The following should no longer be accessed using jaws.X,
	but should instead be ui.X.

	Mark as deprecated.
*/

// Template is an alias for ui.Template.
//
// Deprecated: use ui.Template directly.
//
//go:fix inline
type Template = ui.Template

// RequestWriter is an alias for ui.RequestWriter.
//
// Deprecated: use ui.RequestWriter directly.
//
//go:fix inline
type RequestWriter = ui.RequestWriter

// PathSetter is an alias for ui.PathSetter.
//
// Deprecated: use ui.PathSetter directly.
//
//go:fix inline
type PathSetter = ui.PathSetter

// SetPather is an alias for ui.SetPather.
//
// Deprecated: use ui.SetPather directly.
//
//go:fix inline
type SetPather = ui.SetPather

// JsVar is an alias for ui.JsVar.
//
// Deprecated: use ui.JsVar directly.
//
//go:fix inline
type JsVar[T any] = ui.JsVar[T]

// IsJsVar is an alias for ui.IsJsVar.
//
// Deprecated: use ui.IsJsVar directly.
//
//go:fix inline
type IsJsVar = ui.IsJsVar

// JsVarMaker is an alias for ui.JsVarMaker.
//
// Deprecated: use ui.JsVarMaker directly.
//
//go:fix inline
type JsVarMaker = ui.JsVarMaker

// With is an alias for ui.With.
//
// Deprecated: use ui.With directly.
//
//go:fix inline
type With = ui.With

// NewTemplate creates a new ui.Template.
//
// Deprecated: use ui.NewTemplate directly.
//
//go:fix inline
func NewTemplate(name string, dot any) Template {
	return ui.NewTemplate(name, dot)
}

// NewJsVar creates a new ui.JsVar.
//
// Deprecated: use ui.NewJsVar directly.
//
//go:fix inline
func NewJsVar[T any](l sync.Locker, v *T) *JsVar[T] {
	return ui.NewJsVar(l, v)
}

// UiA is an alias for ui.A.
//
// Deprecated: use ui.A directly.
//
//go:fix inline
type UiA = ui.A

// UiButton is an alias for ui.Button.
//
// Deprecated: use ui.Button directly.
//
//go:fix inline
type UiButton = ui.Button

// UiCheckbox is an alias for ui.Checkbox.
//
// Deprecated: use ui.Checkbox directly.
//
//go:fix inline
type UiCheckbox = ui.Checkbox

// UiContainer is an alias for ui.Container.
//
// Deprecated: use ui.Container directly.
//
//go:fix inline
type UiContainer = ui.Container

// UiDate is an alias for ui.Date.
//
// Deprecated: use ui.Date directly.
//
//go:fix inline
type UiDate = ui.Date

// UiDiv is an alias for ui.Div.
//
// Deprecated: use ui.Div directly.
//
//go:fix inline
type UiDiv = ui.Div

// UiImg is an alias for ui.Img.
//
// Deprecated: use ui.Img directly.
//
//go:fix inline
type UiImg = ui.Img

// UiLabel is an alias for ui.Label.
//
// Deprecated: use ui.Label directly.
//
//go:fix inline
type UiLabel = ui.Label

// UiLi is an alias for ui.Li.
//
// Deprecated: use ui.Li directly.
//
//go:fix inline
type UiLi = ui.Li

// UiNumber is an alias for ui.Number.
//
// Deprecated: use ui.Number directly.
//
//go:fix inline
type UiNumber = ui.Number

// UiPassword is an alias for ui.Password.
//
// Deprecated: use ui.Password directly.
//
//go:fix inline
type UiPassword = ui.Password

// UiRadio is an alias for ui.Radio.
//
// Deprecated: use ui.Radio directly.
//
//go:fix inline
type UiRadio = ui.Radio

// UiRange is an alias for ui.Range.
//
// Deprecated: use ui.Range directly.
//
//go:fix inline
type UiRange = ui.Range

// UiSelect is an alias for ui.Select.
//
// Deprecated: use ui.Select directly.
//
//go:fix inline
type UiSelect = ui.Select

// UiSpan is an alias for ui.Span.
//
// Deprecated: use ui.Span directly.
//
//go:fix inline
type UiSpan = ui.Span

// UiTbody is an alias for ui.Tbody.
//
// Deprecated: use ui.Tbody directly.
//
//go:fix inline
type UiTbody = ui.Tbody

// UiTd is an alias for ui.Td.
//
// Deprecated: use ui.Td directly.
//
//go:fix inline
type UiTd = ui.Td

// UiText is an alias for ui.Text.
//
// Deprecated: use ui.Text directly.
//
//go:fix inline
type UiText = ui.Text

// UiTr is an alias for ui.Tr.
//
// Deprecated: use ui.Tr directly.
//
//go:fix inline
type UiTr = ui.Tr

// NewUiA creates a new ui.A.
//
// Deprecated: use ui.NewA directly.
//
//go:fix inline
func NewUiA(innerHTML HTMLGetter) *UiA {
	return ui.NewA(innerHTML)
}

// NewUiButton creates a new ui.Button.
//
// Deprecated: use ui.NewButton directly.
//
//go:fix inline
func NewUiButton(innerHTML HTMLGetter) *UiButton {
	return ui.NewButton(innerHTML)
}

// NewUiContainer creates a new ui.Container.
//
// Deprecated: use ui.NewContainer directly.
//
//go:fix inline
func NewUiContainer(outerHTMLTag string, c Container) *UiContainer {
	return ui.NewContainer(outerHTMLTag, c)
}

// NewUiDiv creates a new ui.Div.
//
// Deprecated: use ui.NewDiv directly.
//
//go:fix inline
func NewUiDiv(innerHTML HTMLGetter) *UiDiv {
	return ui.NewDiv(innerHTML)
}

// NewUiLabel creates a new ui.Label.
//
// Deprecated: use ui.NewLabel directly.
//
//go:fix inline
func NewUiLabel(innerHTML HTMLGetter) *UiLabel {
	return ui.NewLabel(innerHTML)
}

// NewUiLi creates a new ui.Li.
//
// Deprecated: use ui.NewLi directly.
//
//go:fix inline
func NewUiLi(innerHTML HTMLGetter) *UiLi {
	return ui.NewLi(innerHTML)
}

// NewUiSelect creates a new ui.Select.
//
// Deprecated: use ui.NewSelect directly.
//
//go:fix inline
func NewUiSelect(sh SelectHandler) *UiSelect {
	return ui.NewSelect(sh)
}

// NewUiSpan creates a new ui.Span.
//
// Deprecated: use ui.NewSpan directly.
//
//go:fix inline
func NewUiSpan(innerHTML HTMLGetter) *UiSpan {
	return ui.NewSpan(innerHTML)
}

// NewUiTbody creates a new ui.Tbody.
//
// Deprecated: use ui.NewTbody directly.
//
//go:fix inline
func NewUiTbody(c Container) *UiTbody {
	return ui.NewTbody(c)
}

// NewUiTd creates a new ui.Td.
//
// Deprecated: use ui.NewTd directly.
//
//go:fix inline
func NewUiTd(innerHTML HTMLGetter) *UiTd {
	return ui.NewTd(innerHTML)
}

// NewUiTr creates a new ui.Tr.
//
// Deprecated: use ui.NewTr directly.
//
//go:fix inline
func NewUiTr(innerHTML HTMLGetter) *UiTr {
	return ui.NewTr(innerHTML)
}

// NewUiCheckbox creates a new ui.Checkbox.
//
// Deprecated: use ui.NewCheckbox directly.
//
//go:fix inline
func NewUiCheckbox(g Setter[bool]) *UiCheckbox {
	return ui.NewCheckbox(g)
}

// NewUiDate creates a new ui.Date.
//
// Deprecated: use ui.NewDate directly.
//
//go:fix inline
func NewUiDate(g Setter[time.Time]) *UiDate {
	return ui.NewDate(g)
}

// NewUiImg creates a new ui.Img.
//
// Deprecated: use ui.NewImg directly.
//
//go:fix inline
func NewUiImg(g Getter[string]) *UiImg {
	return ui.NewImg(g)
}

// NewUiNumber creates a new ui.Number.
//
// Deprecated: use ui.NewNumber directly.
//
//go:fix inline
func NewUiNumber(g Setter[float64]) *UiNumber {
	return ui.NewNumber(g)
}

// NewUiPassword creates a new ui.Password.
//
// Deprecated: use ui.NewPassword directly.
//
//go:fix inline
func NewUiPassword(g Setter[string]) *UiPassword {
	return ui.NewPassword(g)
}

// NewUiRadio creates a new ui.Radio.
//
// Deprecated: use ui.NewRadio directly.
//
//go:fix inline
func NewUiRadio(vp Setter[bool]) *UiRadio {
	return ui.NewRadio(vp)
}

// NewUiRange creates a new ui.Range.
//
// Deprecated: use ui.NewRange directly.
//
//go:fix inline
func NewUiRange(g Setter[float64]) *UiRange {
	return ui.NewRange(g)
}

// NewUiText creates a new ui.Text.
//
// Deprecated: use ui.NewText directly.
//
//go:fix inline
func NewUiText(vp Setter[string]) *UiText {
	return ui.NewText(vp)
}
