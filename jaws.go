package jaws

import (
	"sync"
	"time"

	pkg "github.com/linkdata/jaws/jaws"
	"github.com/linkdata/jaws/jid"
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
	RequestWriter        = pkg.RequestWriter
	With                 = pkg.With
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
	UiA         = pkg.UiA
	UiButton    = pkg.UiButton
	UiCheckbox  = pkg.UiCheckbox
	UiContainer = pkg.UiContainer
	UiDate      = pkg.UiDate
	UiDiv       = pkg.UiDiv
	UiImg       = pkg.UiImg
	UiLabel     = pkg.UiLabel
	UiLi        = pkg.UiLi
	UiNumber    = pkg.UiNumber
	UiPassword  = pkg.UiPassword
	UiRadio     = pkg.UiRadio
	UiRange     = pkg.UiRange
	UiSelect    = pkg.UiSelect
	UiSpan      = pkg.UiSpan
	UiTbody     = pkg.UiTbody
	UiTd        = pkg.UiTd
	UiText      = pkg.UiText
	UiTr        = pkg.UiTr
)

// UI constructor assignments (generic types require wrappers, others are direct)
var (
	NewUiA         = pkg.NewUiA
	NewUiButton    = pkg.NewUiButton
	NewUiContainer = pkg.NewUiContainer
	NewUiDiv       = pkg.NewUiDiv
	NewUiLabel     = pkg.NewUiLabel
	NewUiLi        = pkg.NewUiLi
	NewUiSelect    = pkg.NewUiSelect
	NewUiSpan      = pkg.NewUiSpan
	NewUiTbody     = pkg.NewUiTbody
	NewUiTd        = pkg.NewUiTd
	NewUiTr        = pkg.NewUiTr
	NewTestRequest = pkg.NewTestRequest
)

// UI constructors with generic parameters must be wrapped
func NewUiCheckbox(g Setter[bool]) *UiCheckbox {
	return pkg.NewUiCheckbox(g)
}

func NewUiDate(g Setter[time.Time]) *UiDate {
	return pkg.NewUiDate(g)
}

func NewUiImg(g Getter[string]) *UiImg {
	return pkg.NewUiImg(g)
}

func NewUiNumber(g Setter[float64]) *UiNumber {
	return pkg.NewUiNumber(g)
}

func NewUiPassword(g Setter[string]) *UiPassword {
	return pkg.NewUiPassword(g)
}

func NewUiRadio(vp Setter[bool]) *UiRadio {
	return pkg.NewUiRadio(vp)
}

func NewUiRange(g Setter[float64]) *UiRange {
	return pkg.NewUiRange(g)
}

func NewUiText(vp Setter[string]) *UiText {
	return pkg.NewUiText(vp)
}
