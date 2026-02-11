package jaws

import (
	"sync"
	"time"

	pkg "github.com/linkdata/jaws/jaws"
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
	Jaws                 = pkg.Jaws
	Request              = pkg.Request
	Element              = pkg.Element
	UI                   = pkg.UI
	Updater              = pkg.Updater
	Renderer             = pkg.Renderer
	TemplateLookuper     = pkg.TemplateLookuper
	HandleFunc           = pkg.HandleFunc
	PathSetter           = ui.PathSetter
	SetPather            = ui.SetPather
	Formatter            = pkg.Formatter
	Auth                 = pkg.Auth
	InitHandler          = pkg.InitHandler
	ClickHandler         = pkg.ClickHandler
	EventHandler         = pkg.EventHandler
	SelectHandler        = pkg.SelectHandler
	Container            = pkg.Container
	Getter[T comparable] = pkg.Getter[T]
	Setter[T comparable] = pkg.Setter[T]
	Binder[T comparable] = pkg.Binder[T]
	HTMLGetter           = pkg.HTMLGetter
	JsVar[T any]         = ui.JsVar[T]
	IsJsVar              = ui.IsJsVar
	JsVarMaker           = ui.JsVarMaker
	Logger               = pkg.Logger
	RWLocker             = pkg.RWLocker
	TagGetter            = pkg.TagGetter
	NamedBool            = pkg.NamedBool
	NamedBoolArray       = pkg.NamedBoolArray
	Template             = ui.Template
	RequestWriter        = ui.RequestWriter
	With                 = ui.With
	Session              = pkg.Session
	Tag                  = pkg.Tag
	TestRequest          = pkg.TestRequest
)

var (
	ErrEventUnhandled        = pkg.ErrEventUnhandled
	ErrIllegalTagType        = pkg.ErrIllegalTagType // ErrIllegalTagType is returned when a UI tag type is disallowed
	ErrMissingTemplate       = ui.ErrMissingTemplate
	ErrNotComparable         = pkg.ErrNotComparable
	ErrNoWebSocketRequest    = pkg.ErrNoWebSocketRequest
	ErrPendingCancelled      = pkg.ErrPendingCancelled
	ErrValueUnchanged        = pkg.ErrValueUnchanged
	ErrValueNotSettable      = pkg.ErrValueNotSettable
	ErrRequestAlreadyClaimed = pkg.ErrRequestAlreadyClaimed
	ErrJavascriptDisabled    = pkg.ErrJavascriptDisabled
	ErrTooManyTags           = pkg.ErrTooManyTags
)

const (
	ISO8601 = pkg.ISO8601
)

// Non-generic function assignments (no wrapper overhead)
var (
	New               = pkg.New
	JawsKeyString     = pkg.JawsKeyString
	WriteHTMLTag      = pkg.WriteHTMLTag
	NewTemplate       = ui.NewTemplate
	HTMLGetterFunc    = pkg.HTMLGetterFunc
	StringGetterFunc  = pkg.StringGetterFunc
	MakeHTMLGetter    = pkg.MakeHTMLGetter
	NewNamedBool      = pkg.NewNamedBool
	NewNamedBoolArray = pkg.NewNamedBoolArray
	NewTestRequest    = pkg.NewTestRequest
)

// Generic functions must be wrapped
func Bind[T comparable](l sync.Locker, p *T) Binder[T] {
	return pkg.Bind(l, p)
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
