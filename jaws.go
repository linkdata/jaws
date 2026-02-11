package jaws

import (
	"sync"
	"time"

	pkg "github.com/linkdata/jaws/jaws"
	"github.com/linkdata/jaws/jid"
	uipkg "github.com/linkdata/jaws/ui"
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
	PathSetter           = pkg.PathSetter
	SetPather            = pkg.SetPather
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
	JsVar[T any]         = pkg.JsVar[T]
	IsJsVar              = pkg.IsJsVar
	JsVarMaker           = pkg.JsVarMaker
	Logger               = pkg.Logger
	RWLocker             = pkg.RWLocker
	TagGetter            = pkg.TagGetter
	NamedBool            = pkg.NamedBool
	NamedBoolArray       = pkg.NamedBoolArray
	Template             = pkg.Template
	RequestWriter        = uipkg.RequestWriter
	With                 = uipkg.With
	Session              = pkg.Session
	Tag                  = pkg.Tag
	TestRequest          = pkg.TestRequest
)

var (
	ErrEventUnhandled        = pkg.ErrEventUnhandled
	ErrIllegalTagType        = pkg.ErrIllegalTagType // ErrIllegalTagType is returned when a UI tag type is disallowed
	ErrMissingTemplate       = pkg.ErrMissingTemplate
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
	NewTemplate       = pkg.NewTemplate
	HTMLGetterFunc    = pkg.HTMLGetterFunc
	StringGetterFunc  = pkg.StringGetterFunc
	MakeHTMLGetter    = pkg.MakeHTMLGetter
	NewNamedBool      = pkg.NewNamedBool
	NewNamedBoolArray = pkg.NewNamedBoolArray
)

// Generic functions must be wrapped
func Bind[T comparable](l sync.Locker, p *T) Binder[T] {
	return pkg.Bind(l, p)
}

func NewJsVar[T any](l sync.Locker, v *T) *JsVar[T] {
	return pkg.NewJsVar(l, v)
}

type (
	UiA         = uipkg.A
	UiButton    = uipkg.Button
	UiCheckbox  = uipkg.Checkbox
	UiContainer = uipkg.Container
	UiDate      = uipkg.Date
	UiDiv       = uipkg.Div
	UiImg       = uipkg.Img
	UiLabel     = uipkg.Label
	UiLi        = uipkg.Li
	UiNumber    = uipkg.Number
	UiPassword  = uipkg.Password
	UiRadio     = uipkg.Radio
	UiRange     = uipkg.Range
	UiSelect    = uipkg.Select
	UiSpan      = uipkg.Span
	UiTbody     = uipkg.Tbody
	UiTd        = uipkg.Td
	UiText      = uipkg.Text
	UiTr        = uipkg.Tr
)

// UI constructor assignments (generic types require wrappers, others are direct)
var (
	NewUiA         = uipkg.NewA
	NewUiButton    = uipkg.NewButton
	NewUiContainer = uipkg.NewContainer
	NewUiDiv       = uipkg.NewDiv
	NewUiLabel     = uipkg.NewLabel
	NewUiLi        = uipkg.NewLi
	NewUiSelect    = uipkg.NewSelect
	NewUiSpan      = uipkg.NewSpan
	NewUiTbody     = uipkg.NewTbody
	NewUiTd        = uipkg.NewTd
	NewUiTr        = uipkg.NewTr
	NewTestRequest = pkg.NewTestRequest
)

// UI constructors with generic parameters must be wrapped
func NewUiCheckbox(g Setter[bool]) *UiCheckbox {
	return uipkg.NewCheckbox(g)
}

func NewUiDate(g Setter[time.Time]) *UiDate {
	return uipkg.NewDate(g)
}

func NewUiImg(g Getter[string]) *UiImg {
	return uipkg.NewImg(g)
}

func NewUiNumber(g Setter[float64]) *UiNumber {
	return uipkg.NewNumber(g)
}

func NewUiPassword(g Setter[string]) *UiPassword {
	return uipkg.NewPassword(g)
}

func NewUiRadio(vp Setter[bool]) *UiRadio {
	return uipkg.NewRadio(vp)
}

func NewUiRange(g Setter[float64]) *UiRange {
	return uipkg.NewRange(g)
}

func NewUiText(vp Setter[string]) *UiText {
	return uipkg.NewText(vp)
}
