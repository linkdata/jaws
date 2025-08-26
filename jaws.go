package jaws

// The point of this is to not have a zillion files in the repository root.

import (
	"html/template"
	"io"
	"sync"
	"time"

	pkg "github.com/linkdata/jaws/jaws"
	"github.com/linkdata/jaws/jid"
)

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

// New returns a new JaWS object.
// This is expected to be created once per HTTP server and handles
// publishing HTML changes across all connections.
func New() (jw *Jaws, err error) {
	return pkg.New()
}

// NextID returns a uint64 unique within lifetime of the program.
func NextID() int64 {
	return pkg.NextID()
}

// AppendID appends the result of NextID() in text form to the given slice.
func AppendID(b []byte) []byte {
	return pkg.AppendID(b)
}

// MakeID returns a string in the form 'jaws.X' where X is a unique string within lifetime of the program.
func MakeID() string {
	return pkg.MakeID()
}

func JawsKeyString(jawsKey uint64) string {
	return pkg.JawsKeyString(jawsKey)
}

func WriteHTMLTag(w io.Writer, jid jid.Jid, htmlTag string, typeAttr string, valueAttr string, attrs []template.HTMLAttr) (err error) {
	return pkg.WriteHTMLTag(w, jid, htmlTag, typeAttr, valueAttr, attrs)
}

// Bind returns a Binder[T] with the given sync.Locker (or RWLocker) and a pointer to the underlying value of type T.
//
// The pointer will be used as the UI tag.
func Bind[T comparable](l sync.Locker, p *T) Binder[T] {
	return pkg.Bind(l, p)
}

// NewJsVar creates a binding with a Locker (or RWLocker) and
// pointer to underlying data.
//
// JsVar's use JawsRender, and that rendering will contain the
// JSON representation of the underlying data unless it is the
// zero value. If so, it will be used to initialize the named
// Javascript variable before "DOMContentLoaded" fires.
// Note that we don't render the Javascript variable declaration,
// you'll have to do that yourself.
//
// JsVar's do *NOT* use JawsUpdate, so changing the underlying data and
// calling JawsUpdate will have no effect. Instead, JsVar's are
// synchronized across browsers using immediate broadcasts.
//
// Changes to JsVar's should be made using their [JawsSet] or
// [JawsSetPath] methods. If *T implements [PathSetter],
// that will be used instead of jq.Set().
func NewJsVar[T any](l sync.Locker, v *T) *JsVar[T] {
	return pkg.NewJsVar(l, v)
}

func NewNamedBool(nba *NamedBoolArray, name string, html template.HTML, checked bool) *NamedBool {
	return pkg.NewNamedBool(nba, name, html, checked)
}

// NewNamedBoolArray creates a new object to track a related set of named booleans.
//
// The JaWS ID string 'jid' is used as the ID for <select> elements and the
// value for the 'name' attribute for radio buttons. If left empty, MakeID() will
// be used to assign a unique ID.
func NewNamedBoolArray() *NamedBoolArray {
	return pkg.NewNamedBoolArray()
}

// NewTemplate simply returns a Template{} with the members set.
//
// Provided as convenience so as to not have to name the structure members.
func NewTemplate(name string, dot any) Template {
	return pkg.NewTemplate(name, dot)
}

// HTMLGetterFunc wraps a function and returns a HTMLGetter.
func HTMLGetterFunc(fn func(elem *Element) (tmpl template.HTML), tags ...any) HTMLGetter {
	return pkg.HTMLGetterFunc(fn)
}

// UI constructors

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

func NewUiA(innerHTML HTMLGetter) *UiA {
	return pkg.NewUiA(innerHTML)
}
func NewUiButton(innerHTML HTMLGetter) *UiButton {
	return pkg.NewUiButton(innerHTML)
}
func NewUiCheckbox(g Setter[bool]) *UiCheckbox {
	return pkg.NewUiCheckbox(g)
}
func NewUiContainer(outerHTMLTag string, c Container) *UiContainer {
	return pkg.NewUiContainer(outerHTMLTag, c)
}
func NewUiDate(g Setter[time.Time]) *UiDate {
	return pkg.NewUiDate(g)
}
func NewUiDiv(innerHTML HTMLGetter) *UiDiv {
	return pkg.NewUiDiv(innerHTML)
}
func NewUiImg(g Getter[string]) *UiImg {
	return pkg.NewUiImg(g)
}
func NewUiLabel(innerHTML HTMLGetter) *UiLabel {
	return pkg.NewUiLabel(innerHTML)
}
func NewUiLi(innerHTML HTMLGetter) *UiLi {
	return pkg.NewUiLi(innerHTML)
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
func NewUiSelect(sh SelectHandler) *UiSelect {
	return pkg.NewUiSelect(sh)
}
func NewUiSpan(innerHTML HTMLGetter) *UiSpan {
	return pkg.NewUiSpan(innerHTML)
}
func NewUiTbody(c Container) *UiTbody {
	return pkg.NewUiTbody(c)
}
func NewUiTd(innerHTML HTMLGetter) *UiTd {
	return pkg.NewUiTd(innerHTML)
}
func NewUiText(vp Setter[string]) (ui *UiText) {
	return pkg.NewUiText(vp)
}
func NewUiTr(innerHTML HTMLGetter) *UiTr {
	return pkg.NewUiTr(innerHTML)
}
