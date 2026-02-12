package jaws

import (
	"sync"
	"time"

	"github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/ui"
)

// The point of this is to not have a zillion files in the repository root
// while keeping the import path unchanged.
//
// Most exports use direct assignment to avoid wrapper overhead.
// Generic functions must be wrapped since they cannot be assigned without instantiation.

type (
	Jid                  = jid.Jid
	Jaws                 = core.Jaws
	Request              = core.Request
	Element              = core.Element
	UI                   = core.UI
	Updater              = core.Updater
	Renderer             = core.Renderer
	TemplateLookuper     = core.TemplateLookuper
	HandleFunc           = core.HandleFunc
	Formatter            = core.Formatter
	Auth                 = core.Auth
	InitHandler          = core.InitHandler
	ClickHandler         = core.ClickHandler
	EventHandler         = core.EventHandler
	SelectHandler        = core.SelectHandler
	Container            = core.Container
	Getter[T comparable] = core.Getter[T]
	Setter[T comparable] = core.Setter[T]
	Binder[T comparable] = core.Binder[T]
	HTMLGetter           = core.HTMLGetter
	Logger               = core.Logger
	RWLocker             = core.RWLocker
	TagGetter            = core.TagGetter
	NamedBool            = core.NamedBool
	NamedBoolArray       = core.NamedBoolArray
	Session              = core.Session
	Tag                  = core.Tag
	TestRequest          = core.TestRequest
)

var (
	ErrEventUnhandled        = core.ErrEventUnhandled
	ErrIllegalTagType        = core.ErrIllegalTagType // ErrIllegalTagType is returned when a UI tag type is disallowed
	ErrNotComparable         = core.ErrNotComparable
	ErrNoWebSocketRequest    = core.ErrNoWebSocketRequest
	ErrPendingCancelled      = core.ErrPendingCancelled
	ErrValueUnchanged        = core.ErrValueUnchanged
	ErrValueNotSettable      = core.ErrValueNotSettable
	ErrRequestAlreadyClaimed = core.ErrRequestAlreadyClaimed
	ErrJavascriptDisabled    = core.ErrJavascriptDisabled
	ErrTooManyTags           = core.ErrTooManyTags
)

const (
	ISO8601 = core.ISO8601
)

// Non-generic function assignments (no wrapper overhead)
var (
	New               = core.New
	JawsKeyString     = core.JawsKeyString
	WriteHTMLTag      = core.WriteHTMLTag
	HTMLGetterFunc    = core.HTMLGetterFunc
	StringGetterFunc  = core.StringGetterFunc
	MakeHTMLGetter    = core.MakeHTMLGetter
	NewNamedBool      = core.NewNamedBool
	NewNamedBoolArray = core.NewNamedBoolArray
	NewTestRequest    = core.NewTestRequest
)

// Generic functions must be wrapped
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
type Template = ui.Template

// RequestWriter is an alias for ui.RequestWriter.
//
// Deprecated: use ui.RequestWriter directly.
type RequestWriter = ui.RequestWriter

// PathSetter is an alias for ui.PathSetter.
//
// Deprecated: use ui.PathSetter directly.
type PathSetter = ui.PathSetter

// SetPather is an alias for ui.SetPather.
//
// Deprecated: use ui.SetPather directly.
type SetPather = ui.SetPather

// JsVar is an alias for ui.JsVar.
//
// Deprecated: use ui.JsVar directly.
type JsVar[T any] = ui.JsVar[T]

// IsJsVar is an alias for ui.IsJsVar.
//
// Deprecated: use ui.IsJsVar directly.
type IsJsVar = ui.IsJsVar

// JsVarMaker is an alias for ui.JsVarMaker.
//
// Deprecated: use ui.JsVarMaker directly.
type JsVarMaker = ui.JsVarMaker

// With is an alias for ui.With.
//
// Deprecated: use ui.With directly.
type With = ui.With

// NewTemplate creates a new ui.Template.
//
// Deprecated: use ui.NewTemplate directly.
var NewTemplate = ui.NewTemplate

// NewJsVar creates a new ui.JsVar.
//
// Deprecated: use ui.NewJsVar directly.
func NewJsVar[T any](l sync.Locker, v *T) *JsVar[T] {
	return ui.NewJsVar(l, v)
}

// UiA is an alias for ui.A.
//
// Deprecated: use ui.A directly.
type UiA = ui.A

// UiButton is an alias for ui.Button.
//
// Deprecated: use ui.Button directly.
type UiButton = ui.Button

// UiCheckbox is an alias for ui.Checkbox.
//
// Deprecated: use ui.Checkbox directly.
type UiCheckbox = ui.Checkbox

// UiContainer is an alias for ui.Container.
//
// Deprecated: use ui.Container directly.
type UiContainer = ui.Container

// UiDate is an alias for ui.Date.
//
// Deprecated: use ui.Date directly.
type UiDate = ui.Date

// UiDiv is an alias for ui.Div.
//
// Deprecated: use ui.Div directly.
type UiDiv = ui.Div

// UiImg is an alias for ui.Img.
//
// Deprecated: use ui.Img directly.
type UiImg = ui.Img

// UiLabel is an alias for ui.Label.
//
// Deprecated: use ui.Label directly.
type UiLabel = ui.Label

// UiLi is an alias for ui.Li.
//
// Deprecated: use ui.Li directly.
type UiLi = ui.Li

// UiNumber is an alias for ui.Number.
//
// Deprecated: use ui.Number directly.
type UiNumber = ui.Number

// UiPassword is an alias for ui.Password.
//
// Deprecated: use ui.Password directly.
type UiPassword = ui.Password

// UiRadio is an alias for ui.Radio.
//
// Deprecated: use ui.Radio directly.
type UiRadio = ui.Radio

// UiRange is an alias for ui.Range.
//
// Deprecated: use ui.Range directly.
type UiRange = ui.Range

// UiSelect is an alias for ui.Select.
//
// Deprecated: use ui.Select directly.
type UiSelect = ui.Select

// UiSpan is an alias for ui.Span.
//
// Deprecated: use ui.Span directly.
type UiSpan = ui.Span

// UiTbody is an alias for ui.Tbody.
//
// Deprecated: use ui.Tbody directly.
type UiTbody = ui.Tbody

// UiTd is an alias for ui.Td.
//
// Deprecated: use ui.Td directly.
type UiTd = ui.Td

// UiText is an alias for ui.Text.
//
// Deprecated: use ui.Text directly.
type UiText = ui.Text

// UiTr is an alias for ui.Tr.
//
// Deprecated: use ui.Tr directly.
type UiTr = ui.Tr

// NewUiA creates a new ui.A.
//
// Deprecated: use ui.NewA directly.
var NewUiA = ui.NewA

// NewUiButton creates a new ui.Button.
//
// Deprecated: use ui.NewButton directly.
var NewUiButton = ui.NewButton

// NewUiContainer creates a new ui.Container.
//
// Deprecated: use ui.NewContainer directly.
var NewUiContainer = ui.NewContainer

// NewUiDiv creates a new ui.Div.
//
// Deprecated: use ui.NewDiv directly.
var NewUiDiv = ui.NewDiv

// NewUiLabel creates a new ui.Label.
//
// Deprecated: use ui.NewLabel directly.
var NewUiLabel = ui.NewLabel

// NewUiLi creates a new ui.Li.
//
// Deprecated: use ui.NewLi directly.
var NewUiLi = ui.NewLi

// NewUiSelect creates a new ui.Select.
//
// Deprecated: use ui.NewSelect directly.
var NewUiSelect = ui.NewSelect

// NewUiSpan creates a new ui.Span.
//
// Deprecated: use ui.NewSpan directly.
var NewUiSpan = ui.NewSpan

// NewUiTbody creates a new ui.Tbody.
//
// Deprecated: use ui.NewTbody directly.
var NewUiTbody = ui.NewTbody

// NewUiTd creates a new ui.Td.
//
// Deprecated: use ui.NewTd directly.
var NewUiTd = ui.NewTd

// NewUiTr creates a new ui.Tr.
//
// Deprecated: use ui.NewTr directly.
var NewUiTr = ui.NewTr

// NewUiCheckbox creates a new ui.Checkbox.
//
// Deprecated: use ui.NewCheckbox directly.
func NewUiCheckbox(g Setter[bool]) *UiCheckbox {
	return ui.NewCheckbox(g)
}

// NewUiDate creates a new ui.Date.
//
// Deprecated: use ui.NewDate directly.
func NewUiDate(g Setter[time.Time]) *UiDate {
	return ui.NewDate(g)
}

// NewUiImg creates a new ui.Img.
//
// Deprecated: use ui.NewImg directly.
func NewUiImg(g Getter[string]) *UiImg {
	return ui.NewImg(g)
}

// NewUiNumber creates a new ui.Number.
//
// Deprecated: use ui.NewNumber directly.
func NewUiNumber(g Setter[float64]) *UiNumber {
	return ui.NewNumber(g)
}

// NewUiPassword creates a new ui.Password.
//
// Deprecated: use ui.NewPassword directly.
func NewUiPassword(g Setter[string]) *UiPassword {
	return ui.NewPassword(g)
}

// NewUiRadio creates a new ui.Radio.
//
// Deprecated: use ui.NewRadio directly.
func NewUiRadio(vp Setter[bool]) *UiRadio {
	return ui.NewRadio(vp)
}

// NewUiRange creates a new ui.Range.
//
// Deprecated: use ui.NewRange directly.
func NewUiRange(g Setter[float64]) *UiRange {
	return ui.NewRange(g)
}

// NewUiText creates a new ui.Text.
//
// Deprecated: use ui.NewText directly.
func NewUiText(vp Setter[string]) *UiText {
	return ui.NewText(vp)
}
