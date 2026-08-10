package ui

import (
	"fmt"
	"html/template"
	"io"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

// Range renders an HTML range input.
//
// A Range value must back at most one live [jaws.Element]. Construct distinct
// Range values over the same source to render one bound value more than once.
// Construct a Range with [NewRange]; using its zero value as a widget panics.
// Editable Ranges send live browser input while their thumb moves.
type Range struct {
	Input
	binding numericBinding
}

// NewRange returns a range input widget bound to source.
//
// Predeclared and named integer and floating-point types are parsed and formatted
// at their own width. Integer sources use base-10 integer syntax. If source's
// dynamic type also implements [bind.Setter], the input is editable; rendering
// fails unless source exposes the stable, usable tag described by [Input]. A
// getter-only source renders disabled; when tagged, it continues to receive
// dirty-driven server updates.
//
// A non-finite bound floating-point value cancels the [jaws.Request] with a cause
// matching [jaws.ErrValueNotFinite].
//
// Range emits no min, max, or step attributes, so the browser defaults apply.
// Supply them as render parameters when those defaults do not fit the source domain.
// The browser may clamp or round the displayed value; a browser-adjusted value
// reaches the setter only on a later input event and only when representable by T.
func NewRange[T Numeric](source bind.Getter[T]) *Range {
	return &Range{binding: newNumericBinding(source)}
}

// JawsRender renders the Range as an HTML range input.
func (u *Range) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	source := u.binding.sourceValue()
	if u.binding.writable() {
		if err = validateEditableNumericSource(source); err != nil {
			return
		}
	}
	getterAttrs := u.applyGetterAttrs(elem, source)
	text, err := u.binding.getText(elem)
	if err != nil {
		elem.Cancel(err)
		return nil
	}
	attrs := append(elem.ApplyParams(params), getterAttrs...)
	if !u.binding.writable() {
		attrs = append(attrs, template.HTMLAttr("disabled"))
	}
	if err = htmlio.WriteHTMLInput(w, elem.Jid(), "range", text, attrs); err == nil {
		u.Last.Store(text)
	}
	return
}

// JawsUpdate reconciles the range with its canonical source value.
func (u *Range) JawsUpdate(elem *jaws.Element) {
	updateNumericInput(&u.Input, u.binding, elem, "Range")
}

// JawsInput accepts or rejects a browser-side range value.
//
// Text that is malformed or cannot be represented by the source type is rejected
// without calling the setter or returning an error.
// Accepted and rejected text is reconciled with the binding's formatter. Rejection
// and [jaws.ErrValueUnchanged] update only the originating Element. Getter-only
// Ranges ignore browser input.
func (u *Range) JawsInput(elem *jaws.Element, text string) error {
	return handleNumericInput(&u.Input, u.binding, elem, text)
}

// Range renders an HTML range input for value.
//
// Numeric values and getter-only [bind.Getter] sources render disabled;
// [bind.Setter] sources render editable. It panics for any other value. Supply min,
// max, and step attributes in params when the browser defaults do not cover the
// source's domain. For example:
//
//	rw.Range(floatBinder, `min="0"`, `max="200"`, `step="0.5"`)
func (rw RequestWriter) Range(value any, params ...any) error {
	binding, ok := makeNumericBinding(value)
	if !ok {
		panic(fmt.Errorf("expected numeric value or numeric bind.Getter, not %T", value))
	}
	return rw.NewUI(&Range{binding: binding}, params...)
}
