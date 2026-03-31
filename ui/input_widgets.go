package ui

import (
	"io"
	"strconv"
	"sync/atomic"
	"time"

	core "github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/core/assets"
	"github.com/linkdata/jaws/core/jawsbind"
	"github.com/linkdata/jaws/core/jawshtml"
	"github.com/linkdata/jaws/what"
)

// Input stores common state for interactive input widgets.
// There is one of these per request and input widget.
type Input struct {
	Tag  any
	Last atomic.Value // the last value received from the request
}

func (ui *Input) applyGetter(e *core.Element, getter any) (err error) {
	ui.Tag, err = e.ApplyGetter(getter)
	return
}

func (ui *Input) maybeDirty(e *core.Element, inerr error) (err error) {
	err = applyDirty(ui.Tag, e, inerr)
	return
}

type InputText struct {
	Input
	jawsbind.Setter[string]
}

func (ui *InputText) renderStringInput(e *core.Element, w io.Writer, htmlType string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		v := ui.JawsGet(e)
		ui.Last.Store(v)
		err = jawshtml.WriteHTMLInput(w, e.Jid(), htmlType, v, attrs)
	}
	return
}

func (ui *InputText) JawsUpdate(e *core.Element) {
	if v := ui.JawsGet(e); ui.Last.Swap(v) != v {
		e.SetValue(v)
	}
}

func (ui *InputText) JawsEvent(e *core.Element, wht what.What, val string) (err error) {
	err = core.ErrEventUnhandled
	if wht == what.Input {
		ui.Last.Store(val)
		err = ui.maybeDirty(e, ui.Setter.JawsSet(e, val))
	}
	return
}

type InputBool struct {
	Input
	jawsbind.Setter[bool]
}

func (ui *InputBool) renderBoolInput(e *core.Element, w io.Writer, htmlType string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		v := ui.JawsGet(e)
		ui.Last.Store(v)
		if v {
			attrs = append(attrs, "checked")
		}
		err = jawshtml.WriteHTMLInput(w, e.Jid(), htmlType, "", attrs)
	}
	return
}

func (ui *InputBool) JawsUpdate(e *core.Element) {
	v := ui.JawsGet(e)
	if ui.Last.Swap(v) != v {
		txt := "false"
		if v {
			txt = "true"
		}
		e.SetValue(txt)
	}
}

func (ui *InputBool) JawsEvent(e *core.Element, wht what.What, val string) (err error) {
	err = core.ErrEventUnhandled
	if wht == what.Input {
		if val == "" {
			val = "false"
		}
		var v bool
		if v, err = strconv.ParseBool(val); err == nil {
			err = ui.maybeDirty(e, ui.Setter.JawsSet(e, v))
		}
		ui.Last.Store(v)
	}
	return
}

type InputFloat struct {
	Input
	jawsbind.Setter[float64]
}

func (ui *InputFloat) str(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func (ui *InputFloat) renderFloatInput(e *core.Element, w io.Writer, htmlType string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		v := ui.JawsGet(e)
		ui.Last.Store(v)
		err = jawshtml.WriteHTMLInput(w, e.Jid(), htmlType, ui.str(v), attrs)
	}
	return
}

func (ui *InputFloat) JawsUpdate(e *core.Element) {
	v := ui.JawsGet(e)
	if ui.Last.Swap(v) != v {
		e.SetValue(ui.str(v))
	}
}

func (ui *InputFloat) JawsEvent(e *core.Element, wht what.What, val string) (err error) {
	err = core.ErrEventUnhandled
	if wht == what.Input {
		if val == "" {
			val = "0"
		}
		var v float64
		if v, err = strconv.ParseFloat(val, 64); err == nil {
			err = ui.maybeDirty(e, ui.Setter.JawsSet(e, v))
		}
		ui.Last.Store(v)
	}
	return
}

type InputDate struct {
	Input
	jawsbind.Setter[time.Time]
}

func (ui *InputDate) str(v time.Time) string {
	return v.Format(assets.ISO8601)
}

func (ui *InputDate) renderDateInput(e *core.Element, w io.Writer, htmlType string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		v := ui.JawsGet(e)
		ui.Last.Store(v)
		err = jawshtml.WriteHTMLInput(w, e.Jid(), htmlType, ui.str(v), attrs)
	}
	return
}

func (ui *InputDate) JawsUpdate(e *core.Element) {
	v := ui.JawsGet(e)
	if ui.Last.Swap(v) != v {
		e.SetValue(ui.str(v))
	}
}

func (ui *InputDate) JawsEvent(e *core.Element, wht what.What, val string) (err error) {
	err = core.ErrEventUnhandled
	if wht == what.Input {
		if val == "" {
			val = "0001-01-01"
		}
		var v time.Time
		if v, err = time.Parse(assets.ISO8601, val); err == nil {
			err = ui.maybeDirty(e, ui.Setter.JawsSet(e, v))
		}
		ui.Last.Store(v)
	}
	return
}
