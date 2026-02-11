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
	PathSetter           = ui.PathSetter
	SetPather            = ui.SetPather
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
	JsVar[T any]         = ui.JsVar[T]
	IsJsVar              = ui.IsJsVar
	JsVarMaker           = ui.JsVarMaker
	Logger               = core.Logger
	RWLocker             = core.RWLocker
	TagGetter            = core.TagGetter
	NamedBool            = core.NamedBool
	NamedBoolArray       = core.NamedBoolArray
	Template             = ui.Template
	RequestWriter        = ui.RequestWriter
	With                 = ui.With
	Session              = core.Session
	Tag                  = core.Tag
	TestRequest          = core.TestRequest
)

var (
	ErrEventUnhandled        = core.ErrEventUnhandled
	ErrIllegalTagType        = core.ErrIllegalTagType // ErrIllegalTagType is returned when a UI tag type is disallowed
	ErrMissingTemplate       = ui.ErrMissingTemplate
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
	NewTemplate       = ui.NewTemplate
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

func NewJsVar[T any](l sync.Locker, v *T) *JsVar[T] {
	return ui.NewJsVar(l, v)
}

type (
	UiA         = ui.A
	UiButton    = ui.Button
	UiCheckbox  = ui.Checkbox
	UiContainer = ui.Container
	UiDate      = ui.Date
	UiDiv       = ui.Div
	UiImg       = ui.Img
	UiLabel     = ui.Label
	UiLi        = ui.Li
	UiNumber    = ui.Number
	UiPassword  = ui.Password
	UiRadio     = ui.Radio
	UiRange     = ui.Range
	UiSelect    = ui.Select
	UiSpan      = ui.Span
	UiTbody     = ui.Tbody
	UiTd        = ui.Td
	UiText      = ui.Text
	UiTr        = ui.Tr
)

// UI constructor assignments (generic types require wrappers, others are direct)
var (
	NewUiA         = ui.NewA
	NewUiButton    = ui.NewButton
	NewUiContainer = ui.NewContainer
	NewUiDiv       = ui.NewDiv
	NewUiLabel     = ui.NewLabel
	NewUiLi        = ui.NewLi
	NewUiSelect    = ui.NewSelect
	NewUiSpan      = ui.NewSpan
	NewUiTbody     = ui.NewTbody
	NewUiTd        = ui.NewTd
	NewUiTr        = ui.NewTr
)

// UI constructors with generic parameters must be wrapped
func NewUiCheckbox(g Setter[bool]) *UiCheckbox {
	return ui.NewCheckbox(g)
}

func NewUiDate(g Setter[time.Time]) *UiDate {
	return ui.NewDate(g)
}

func NewUiImg(g Getter[string]) *UiImg {
	return ui.NewImg(g)
}

func NewUiNumber(g Setter[float64]) *UiNumber {
	return ui.NewNumber(g)
}

func NewUiPassword(g Setter[string]) *UiPassword {
	return ui.NewPassword(g)
}

func NewUiRadio(vp Setter[bool]) *UiRadio {
	return ui.NewRadio(vp)
}

func NewUiRange(g Setter[float64]) *UiRange {
	return ui.NewRange(g)
}

func NewUiText(vp Setter[string]) *UiText {
	return ui.NewText(vp)
}
