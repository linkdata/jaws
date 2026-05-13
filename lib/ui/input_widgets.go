package ui

import (
	"html/template"
	"io"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/assets"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

// Input stores common state for interactive input widgets.
// There is one of these per request and input widget.
type Input struct {
	Tag  any          // tag to dirty after accepted input
	Last atomic.Value // the last value received from the request
}

func (u *Input) applyGetterAttrs(elem *jaws.Element, getter any) (attrs []template.HTMLAttr, err error) {
	u.Tag, attrs, err = elem.ApplyGetter(getter)
	return
}

func (u *Input) maybeDirty(elem *jaws.Element, inErr error) (err error) {
	err = applyDirty(u.Tag, elem, inErr)
	return
}

// InputText is the reusable base for string input widgets.
type InputText struct {
	Input
	bind.Setter[string]
}

func (u *InputText) renderStringInput(elem *jaws.Element, w io.Writer, htmlType string, params ...any) (err error) {
	var getterAttrs []template.HTMLAttr
	if getterAttrs, err = u.applyGetterAttrs(elem, u.Setter); err == nil {
		attrs := append(elem.ApplyParams(params), getterAttrs...)
		v := u.JawsGet(elem)
		u.Last.Store(v)
		err = htmlio.WriteHTMLInput(w, elem.Jid(), htmlType, v, attrs)
	}
	return
}

// JawsUpdate updates the input value when the bound string value changes.
func (u *InputText) JawsUpdate(elem *jaws.Element) {
	if v := u.JawsGet(elem); u.Last.Swap(v) != v {
		elem.SetValue(v)
	}
}

// JawsInput stores a browser-side string input value.
func (u *InputText) JawsInput(elem *jaws.Element, value string) (err error) {
	u.Last.Store(value)
	err = u.maybeDirty(elem, u.Setter.JawsSet(elem, value))
	return
}

// InputBool is the reusable base for boolean input widgets.
type InputBool struct {
	Input
	bind.Setter[bool]
}

func (u *InputBool) renderBoolInput(elem *jaws.Element, w io.Writer, htmlType string, params ...any) (err error) {
	var getterAttrs []template.HTMLAttr
	if getterAttrs, err = u.applyGetterAttrs(elem, u.Setter); err == nil {
		attrs := append(elem.ApplyParams(params), getterAttrs...)
		v := u.JawsGet(elem)
		u.Last.Store(v)
		if v {
			attrs = append(attrs, "checked")
		}
		err = htmlio.WriteHTMLInput(w, elem.Jid(), htmlType, "", attrs)
	}
	return
}

// JawsUpdate updates the input value when the bound bool value changes.
func (u *InputBool) JawsUpdate(elem *jaws.Element) {
	v := u.JawsGet(elem)
	if u.Last.Swap(v) != v {
		txt := "false"
		if v {
			txt = "true"
		}
		elem.SetValue(txt)
	}
}

// JawsInput stores a browser-side bool input value.
func (u *InputBool) JawsInput(elem *jaws.Element, value string) (err error) {
	if value == "" {
		value = "false"
	}
	var v bool
	if v, err = strconv.ParseBool(value); err == nil {
		err = u.maybeDirty(elem, u.Setter.JawsSet(elem, v))
	}
	u.Last.Store(v)
	return
}

// InputFloat is the reusable base for float64 input widgets.
type InputFloat struct {
	Input
	bind.Setter[float64]
}

func (u *InputFloat) str(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (u *InputFloat) renderFloatInput(elem *jaws.Element, w io.Writer, htmlType string, params ...any) (err error) {
	var getterAttrs []template.HTMLAttr
	if getterAttrs, err = u.applyGetterAttrs(elem, u.Setter); err == nil {
		attrs := append(elem.ApplyParams(params), getterAttrs...)
		v := u.JawsGet(elem)
		u.Last.Store(v)
		err = htmlio.WriteHTMLInput(w, elem.Jid(), htmlType, u.str(v), attrs)
	}
	return
}

// JawsUpdate updates the input value when the bound float64 value changes.
func (u *InputFloat) JawsUpdate(elem *jaws.Element) {
	v := u.JawsGet(elem)
	if u.Last.Swap(v) != v {
		elem.SetValue(u.str(v))
	}
}

// JawsInput stores a browser-side float64 input value.
func (u *InputFloat) JawsInput(elem *jaws.Element, value string) (err error) {
	if value == "" {
		value = "0"
	}
	var v float64
	if v, err = strconv.ParseFloat(value, 64); err == nil {
		err = u.maybeDirty(elem, u.Setter.JawsSet(elem, v))
	}
	u.Last.Store(v)
	return
}

// InputDate is the reusable base for date input widgets.
type InputDate struct {
	Input
	bind.Setter[time.Time]
}

func (u *InputDate) str(v time.Time) string {
	return v.Format(assets.ISO8601)
}

func (u *InputDate) renderDateInput(elem *jaws.Element, w io.Writer, htmlType string, params ...any) (err error) {
	var getterAttrs []template.HTMLAttr
	if getterAttrs, err = u.applyGetterAttrs(elem, u.Setter); err == nil {
		attrs := append(elem.ApplyParams(params), getterAttrs...)
		v := u.JawsGet(elem)
		u.Last.Store(v)
		err = htmlio.WriteHTMLInput(w, elem.Jid(), htmlType, u.str(v), attrs)
	}
	return
}

// JawsUpdate updates the input value when the bound date value changes.
func (u *InputDate) JawsUpdate(elem *jaws.Element) {
	v := u.JawsGet(elem)
	if u.Last.Swap(v) != v {
		elem.SetValue(u.str(v))
	}
}

// JawsInput stores a browser-side date input value.
func (u *InputDate) JawsInput(elem *jaws.Element, value string) (err error) {
	if value == "" {
		value = "0001-01-01"
	}
	var v time.Time
	if v, err = time.Parse(assets.ISO8601, value); err == nil {
		err = u.maybeDirty(elem, u.Setter.JawsSet(elem, v))
	}
	u.Last.Store(v)
	return
}
