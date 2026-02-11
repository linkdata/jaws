package ui

import (
	"io"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/what"
)

// Input stores common state for interactive input widgets.
type Input struct {
	Tag  any
	Last atomic.Value
}

func (ui *Input) applyGetter(e *core.Element, getter any) (err error) {
	ui.Tag, err = e.ApplyGetter(getter)
	return
}

func (ui *Input) maybeDirty(val any, e *core.Element, err error) error {
	if changed, err := applyDirty(ui.Tag, e, err); err != nil {
		return err
	} else if changed {
		ui.Last.Store(val)
	}
	return nil
}

type InputText struct {
	Input
	core.Setter[string]
}

func (ui *InputText) renderStringInput(e *core.Element, w io.Writer, htmlType string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		v := ui.JawsGet(e)
		ui.Last.Store(v)
		err = core.WriteHTMLInput(w, e.Jid(), htmlType, v, attrs)
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
		err = ui.maybeDirty(val, e, ui.Setter.JawsSet(e, val))
	}
	return
}

type InputBool struct {
	Input
	core.Setter[bool]
}

func (ui *InputBool) renderBoolInput(e *core.Element, w io.Writer, htmlType string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		v := ui.JawsGet(e)
		ui.Last.Store(v)
		if v {
			attrs = append(attrs, "checked")
		}
		err = core.WriteHTMLInput(w, e.Jid(), htmlType, "", attrs)
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
		var v bool
		if val != "" {
			if v, err = strconv.ParseBool(val); err != nil {
				return
			}
		}
		err = ui.maybeDirty(v, e, ui.Setter.JawsSet(e, v))
	}
	return
}

type InputFloat struct {
	Input
	core.Setter[float64]
}

func (ui *InputFloat) str() string {
	return strconv.FormatFloat(ui.Last.Load().(float64), 'f', -1, 64)
}

func (ui *InputFloat) renderFloatInput(e *core.Element, w io.Writer, htmlType string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		ui.Last.Store(ui.JawsGet(e))
		err = core.WriteHTMLInput(w, e.Jid(), htmlType, ui.str(), attrs)
	}
	return
}

func (ui *InputFloat) JawsUpdate(e *core.Element) {
	if f := ui.JawsGet(e); ui.Last.Swap(f) != f {
		e.SetValue(ui.str())
	}
}

func (ui *InputFloat) JawsEvent(e *core.Element, wht what.What, val string) (err error) {
	err = core.ErrEventUnhandled
	if wht == what.Input {
		var v float64
		if val != "" {
			if v, err = strconv.ParseFloat(val, 64); err != nil {
				return
			}
		}
		err = ui.maybeDirty(v, e, ui.Setter.JawsSet(e, v))
	}
	return
}

type InputDate struct {
	Input
	core.Setter[time.Time]
}

func (ui *InputDate) str() string {
	return ui.Last.Load().(time.Time).Format(core.ISO8601)
}

func (ui *InputDate) renderDateInput(e *core.Element, w io.Writer, htmlType string, params ...any) (err error) {
	if err = ui.applyGetter(e, ui.Setter); err == nil {
		attrs := e.ApplyParams(params)
		ui.Last.Store(ui.JawsGet(e))
		err = core.WriteHTMLInput(w, e.Jid(), htmlType, ui.str(), attrs)
	}
	return
}

func (ui *InputDate) JawsUpdate(e *core.Element) {
	if t := ui.JawsGet(e); ui.Last.Swap(t) != t {
		e.SetValue(ui.str())
	}
}

func (ui *InputDate) JawsEvent(e *core.Element, wht what.What, val string) (err error) {
	err = core.ErrEventUnhandled
	if wht == what.Input {
		var v time.Time
		if val != "" {
			if v, err = time.Parse(core.ISO8601, val); err != nil {
				return
			}
		}
		err = ui.maybeDirty(v, e, ui.Setter.JawsSet(e, v))
	}
	return
}
