package ui

import (
	"html/template"
	"io"
	"math"
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
	// tag is the dirty tag, written once during render and read on the event
	// goroutine (JawsInput). The render-completes-before-events lifecycle makes
	// the unsynchronized access safe; it is unexported so external code cannot
	// mutate it.
	tag  any
	Last atomic.Value // the last value received from the request
}

func (u *Input) applyGetterAttrs(elem *jaws.Element, getter any) (attrs []template.HTMLAttr, err error) {
	u.tag, attrs, err = elem.ApplyGetter(getter)
	return
}

// maybeDirty applies the dirty state for this widget's bound tag (u.tag),
// forwarding inErr to [applyDirty].
func (u *Input) maybeDirty(elem *jaws.Element, inErr error) (err error) {
	err = applyDirty(u.tag, elem, inErr)
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
	// A string has no parse step that can fail, so Last is stored unconditionally,
	// unlike the typed inputs (InputBool/InputFloat/InputDate) which store Last only
	// after a successful parse to keep the last accepted value on a malformed frame.
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
		// jaws.js applies a what.Value of "true"/"false" to a checkable input's
		// checked state, so send the boolean as that literal text.
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
	// Parse errors are malformed client frames: jaws.js sends boolean values
	// for checkable inputs. Leave Last as the last accepted value.
	if v, err = strconv.ParseBool(value); err == nil {
		u.Last.Store(v)
		err = u.maybeDirty(elem, u.Setter.JawsSet(elem, v))
	}
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
	// An empty Last (no value stored yet, e.g. an update-only Register that never ran
	// renderFloatInput) makes the float64 assertion fail with ok==false; send the
	// initial value unconditionally in that case, matching how the other input
	// widgets' nil != value comparison sends on their first update.
	prev, ok := u.Last.Swap(v).(float64)
	// Compare raw float64 values, not rendered strings: this can skip rare cosmetic
	// changes such as -0 -> 0, but avoids formatting on the common unchanged path.
	// NaN != NaN, so a plain compare would re-send a NaN bound value on every update
	// cycle (JawsInput rejects NaN from the browser, but the server can bind one);
	// treat NaN -> NaN as unchanged. A real transition into or out of NaN still sends.
	if !ok || (prev != v && !(math.IsNaN(prev) && math.IsNaN(v))) {
		elem.SetValue(u.str(v))
	}
}

// JawsInput stores a browser-side float64 input value.
func (u *InputFloat) JawsInput(elem *jaws.Element, value string) (err error) {
	if value == "" {
		// Empty is a normal in-progress edit state for number/range controls:
		// a user replacing "0" with "1" first clears the field. Treat it as zero
		// for the bound value, but do not force an immediate corrective SetValue
		// when the setter reports unchanged; that would race ordinary typing.
		value = "0"
	}
	var v float64
	// Parse errors are malformed client frames: jaws.js reads elem.value from
	// browser number/range controls. Leave Last as the last accepted value.
	if v, err = strconv.ParseFloat(value, 64); err == nil {
		// The browser is untrusted and strconv.ParseFloat accepts "NaN"/"Inf".
		// Reject non-finite input here (mirroring click.go's runAtof) so it never
		// reaches the bound value or u.Last. NaN is especially harmful: it defeats
		// the Last.Swap(v) != v dedup in JawsUpdate (NaN != NaN), which would
		// re-emit a SetValue on every update cycle.
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return bind.ErrFloatNotFinite
		}
		u.Last.Store(v)
		err = u.maybeDirty(elem, u.Setter.JawsSet(elem, v))
	}
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
		// Dedup on the rendered ISO8601 string, not the raw time.Time: comparing
		// time.Time with == also compares the monotonic reading and *Location, so
		// equal calendar dates can compare unequal. The string is what we send.
		u.Last.Store(u.str(v))
		err = htmlio.WriteHTMLInput(w, elem.Jid(), htmlType, u.str(v), attrs)
	}
	return
}

// JawsUpdate updates the input value when the bound date value changes.
func (u *InputDate) JawsUpdate(elem *jaws.Element) {
	if s := u.str(u.JawsGet(elem)); u.Last.Swap(s) != s {
		elem.SetValue(s)
	}
}

// JawsInput stores a browser-side date input value.
func (u *InputDate) JawsInput(elem *jaws.Element, value string) (err error) {
	if value == "" {
		value = "0001-01-01"
	}
	var v time.Time
	// Parse errors are malformed client frames: jaws.js reads elem.value from
	// browser date controls. Leave Last as the last accepted value.
	if v, err = time.Parse(assets.ISO8601, value); err == nil {
		u.Last.Store(u.str(v))
		err = u.maybeDirty(elem, u.Setter.JawsSet(elem, v))
	}
	return
}
