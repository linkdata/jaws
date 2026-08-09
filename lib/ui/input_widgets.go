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
//
// An Input value provides a widget-owned update cache and dirty target for one
// live [jaws.Element]. A widget embedding Input must therefore back at most one
// live Element. To render the same bound state more than once, construct distinct
// widgets that share the setter.
//
// For post-set reconciliation, a writable setter must expose at least one stable,
// usable tag through [jaws.Element.ApplyGetter]. [bind.New] exposes its backing
// value pointer.
//
// After [bind.Setter.JawsSet] returns a result that does not match
// [jaws.ErrValueUnchanged], Input dirties the setter-derived tag so the server
// value can reconcile rejected or normalized browser input. Tags supplied as
// render parameters register the Element but do not replace that dirty target.
// Without a valid setter-derived target, automatic reconciliation does not occur.
type Input struct {
	// tag is the dirty tag, written once during render and read on the event
	// goroutine (JawsInput). The render-completes-before-events lifecycle makes
	// the unsynchronized access safe; it is unexported so external code cannot
	// mutate it.
	tag any
	// Last holds widget-specific state used to filter redundant browser updates.
	// Widget implementations own this cache; callers must not modify it.
	Last atomic.Value
}

func (u *Input) applyGetterAttrs(elem *jaws.Element, getter any) (attrs []template.HTMLAttr) {
	u.tag = elem.ApplyGetter(getter)
	attrs = elem.ApplyInitialHTMLAttr(getter)
	return
}

// maybeDirty applies the dirty state for this widget's bound tag (u.tag),
// forwarding inErr to [applyDirty].
func (u *Input) maybeDirty(elem *jaws.Element, inErr error) (err error) {
	err = applyDirty(u.tag, elem, inErr)
	return
}

// InputText is the reusable base for string input widgets.
//
// A widget embedding InputText must back at most one live [jaws.Element].
type InputText struct {
	Input
	bind.Setter[string]
}

func (u *InputText) renderStringInput(elem *jaws.Element, w io.Writer, htmlType string, params ...any) (err error) {
	getterAttrs := u.applyGetterAttrs(elem, u.Setter)
	attrs := append(elem.ApplyParams(params), getterAttrs...)
	v := u.JawsGet(elem)
	u.Last.Store(v)
	err = htmlio.WriteHTMLInput(w, elem.Jid(), htmlType, v, attrs)
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
	// unlike the typed inputs (InputBool/InputDate) which store Last only
	// after a successful parse to keep the last accepted value on a malformed frame.
	u.Last.Store(value)
	err = u.maybeDirty(elem, u.Setter.JawsSet(elem, value))
	return
}

// InputBool is the reusable base for boolean input widgets.
//
// A widget embedding InputBool must back at most one live [jaws.Element].
type InputBool struct {
	Input
	bind.Setter[bool]
}

func (u *InputBool) renderBoolInput(elem *jaws.Element, w io.Writer, htmlType string, params ...any) (err error) {
	getterAttrs := u.applyGetterAttrs(elem, u.Setter)
	attrs := append(elem.ApplyParams(params), getterAttrs...)
	v := u.JawsGet(elem)
	u.Last.Store(v)
	if v {
		attrs = append(attrs, "checked")
	}
	err = htmlio.WriteHTMLInput(w, elem.Jid(), htmlType, "", attrs)
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

// InputDate is the reusable base for date input widgets.
//
// A widget embedding InputDate must back at most one live [jaws.Element].
//
// The control is date-only. Rendering shows the calendar date in the bound
// value's own location, but a browser edit normalizes the bound [time.Time] to
// midnight UTC of the picked date, and only years 1 through 9999 round-trip; see
// [InputDate.JawsInput].
type InputDate struct {
	Input
	bind.Setter[time.Time]
}

func (u *InputDate) str(v time.Time) string {
	return v.Format(assets.ISO8601)
}

func (u *InputDate) renderDateInput(elem *jaws.Element, w io.Writer, htmlType string, params ...any) (err error) {
	getterAttrs := u.applyGetterAttrs(elem, u.Setter)
	attrs := append(elem.ApplyParams(params), getterAttrs...)
	v := u.JawsGet(elem)
	// Dedup on the rendered ISO8601 string, not the raw time.Time: comparing
	// time.Time with == also compares the monotonic reading and *Location, so
	// equal calendar dates can compare unequal. The string is what we send.
	u.Last.Store(u.str(v))
	err = htmlio.WriteHTMLInput(w, elem.Jid(), htmlType, u.str(v), attrs)
	return
}

// JawsUpdate updates the input value when the bound date value changes.
func (u *InputDate) JawsUpdate(elem *jaws.Element) {
	if s := u.str(u.JawsGet(elem)); u.Last.Swap(s) != s {
		elem.SetValue(s)
	}
}

// JawsInput stores a browser-side date input value.
//
// The browser sends a calendar date (YYYY-MM-DD), which [time.Parse] resolves
// to midnight UTC, so the stored [time.Time] drops any time-of-day and
// [time.Location] the previously bound value carried. In a non-UTC deployment
// the stored instant therefore shifts by the zone offset, and because
// [time.Time] inequality includes the location, re-selecting the same date
// still reports a change and broadcasts it. Bind a date whose clock and zone are
// irrelevant, or keep your bound values at midnight UTC to match.
//
// Only years 1 through 9999 round-trip. Those render as four digits, which is
// what the fixed-width "2006-01-02" layout parses back. A bound year of 10000 or
// more renders with five or more digits, because [time.Time.Format] widens the
// year field, but [time.Parse] then rejects the extra digit; that edit returns a
// parse error and leaves the last accepted value in place instead of updating the
// bound value. Keep bound years within 1..9999.
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
