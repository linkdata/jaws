package ui

import (
	"errors"
	"fmt"
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

// Number renders an HTML number input.
//
// A Number value must back at most one live [jaws.Element]. Construct distinct
// Number values over the same source to render one bound value more than once.
// Construct a Number with [NewNumber] or [NewNumberWith]; using its zero value as a
// widget panics. Number requires ordinary rendering and is not supported by
// [RequestWriter.Register].
//
// Editable Numbers send settled edits on the browser's change event. A pending
// edit is flushed before another JaWS-managed input, click, or context-menu event,
// but JaWS adds no pre-event flush for native or implicit form submission. Server
// and ancestor renders may replace an edit that has not settled.
type Number struct {
	Input
	binding *numericBinding
}

// NewNumber returns a number input widget bound to source.
//
// Predeclared and named integer and floating-point types are parsed and formatted
// at their own width. If source's dynamic type also implements [bind.Setter], the
// input is editable; rendering fails unless source exposes the stable, usable tag
// described by [Input]. A getter-only source renders read-only.
//
// A non-finite bound floating-point value cancels the [jaws.Request] with a cause
// matching [jaws.ErrValueNotFinite].
func NewNumber[T Numeric](source bind.Getter[T]) *Number {
	return newNumber(newBuiltinNumericBinding(source))
}

// NewNumberWith returns a number input widget using codec.
//
// The source capability and tag rules are the same as for [NewNumber]. Browser
// text is validated before [NumberCodec.ParseNumber] is called; see [NumberCodec]
// for the codec and value requirements. A contract violation makes rendering return
// an error matching [ErrNumberFormat]; during input or update it cancels the
// [jaws.Request] with a matching cause.
func NewNumberWith[T comparable](source bind.Getter[T], codec NumberCodec[T]) *Number {
	return newNumber(newCustomNumericBinding(source, codec))
}

func newNumber(binding *numericBinding) *Number {
	return &Number{binding: binding}
}

// JawsRender renders the Number as an HTML number input.
func (u *Number) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	if u.binding.writable() {
		if err = validateEditableNumericSource(u.binding.source()); err != nil {
			return
		}
	}
	getterAttrs := u.applyGetterAttrs(elem, u.binding.source())
	_, text, err := u.binding.get(elem)
	if err != nil {
		if errors.Is(err, jaws.ErrValueNotFinite) {
			elem.Cancel(err)
			return nil
		}
		return err
	}
	attrs := append(elem.ApplyParams(params), getterAttrs...)
	if u.binding.writable() {
		attrs = append(attrs, template.HTMLAttr("data-jawsnumber"))
	} else {
		attrs = append(attrs, template.HTMLAttr("readonly"))
	}
	if err = htmlio.WriteHTMLInput(w, elem.Jid(), "number", text, attrs); err == nil {
		u.Last.Store(text)
	}
	return
}

// JawsUpdate reconciles the input with its canonical source value.
func (u *Number) JawsUpdate(elem *jaws.Element) {
	if u.Last.Load() == nil {
		elem.Request.MustLog(errors.New("ui.Number.JawsUpdate called before successful rendering"))
		return
	}
	_, text, err := u.binding.get(elem)
	if err != nil {
		elem.Cancel(err)
		return
	}
	if prev := u.Last.Swap(text).(string); prev != text {
		elem.SetValue(text)
	}
}

// JawsInput settles a browser-side number edit.
//
// Empty, malformed, non-finite, and unrepresentable browser values are rejected
// without calling the setter or returning an error. The canonical value is
// restored by updating only the originating Element. Accepted text is also
// rewritten through the binding's formatter, including when the setter returns
// [jaws.ErrValueUnchanged]. Getter-only Numbers ignore browser input.
func (u *Number) JawsInput(elem *jaws.Element, text string) (err error) {
	if !u.binding.writable() {
		return
	}
	value, ok, parseErr := u.binding.parse(text)
	// A formatter can never produce empty text, so empty is the invalidated
	// baseline that forces the next update to send the canonical value.
	u.Last.Store("")
	if parseErr != nil {
		elem.Cancel(parseErr)
		return
	}
	if !ok {
		// Reconcile in JawsUpdate so the Request serializes this correction with
		// source-driven updates and leaves a newer source value final.
		elem.Dirty(elem)
		return
	}
	err = u.binding.set(elem, value)
	if errors.Is(err, jaws.ErrValueUnchanged) {
		elem.Dirty(elem)
		err = nil
		return
	}
	err = u.maybeDirty(elem, err)
	return
}

// Number renders an HTML number input for value.
//
// Numeric values and getter-only [bind.Getter] sources render read-only;
// [bind.Setter] sources render editable. A [NumericBinding] returned by
// [NewNumericBinding] supplies a custom [NumberCodec]. It panics for any other
// value or an unusable NumericBinding.
func (rw RequestWriter) Number(value any, params ...any) error {
	binding, ok := makeNumericBinding(value)
	if !ok {
		panic(fmt.Errorf("expected numeric value or numeric bind.Getter, not %T", value))
	}
	return rw.NewUI(newNumber(binding), params...)
}
