package ui

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"sync/atomic"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/bind"
	"github.com/linkdata/jaws/lib/htmlio"
)

// Range renders an HTML range input.
//
// A Range value must back at most one live [jaws.Element]. Construct distinct
// Range values over the same source to render one bound value more than once.
// Editable Ranges send live browser input while their thumb moves.
type Range struct {
	Input
	binding   *numericBinding
	lastValid atomic.Bool
}

// NewRange returns a range input widget bound to source.
//
// Predeclared and named integer and floating-point types are parsed and formatted
// at their own width. If source's dynamic type also implements [bind.Setter], the
// input is editable and source must expose the stable usable tag described by
// [Input]. A getter-only source renders disabled; when tagged, it continues to
// receive dirty-driven server updates.
//
// A non-finite bound floating-point value cancels the [jaws.Request] with a cause
// matching [jaws.ErrValueNotFinite].
//
// The widget emits no min, max, or step attribute of its own, so the browser
// applies the HTML range defaults min="0", max="100", step="1". A bound value
// outside that domain, or off the step grid, is clamped and rounded by the
// browser. A representable adjusted value is echoed through the setter on the
// next input event; an adjustment not representable by T is rejected. Supply
// explicit attributes when the defaults do not cover the source's domain.
func NewRange[T Numeric](source bind.Getter[T]) *Range {
	return newRange(newBuiltinNumericBinding(source))
}

func newRange(binding *numericBinding) *Range {
	return &Range{binding: binding}
}

// JawsRender renders ui as an HTML range input.
func (u *Range) JawsRender(elem *jaws.Element, w io.Writer, params []any) (err error) {
	if u.binding.writable() {
		if err = validateEditableNumericSource(u.binding.source()); err != nil {
			return
		}
	}
	getterAttrs := u.applyGetterAttrs(elem, u.binding.source())
	value, text, err := u.binding.get(elem)
	if err != nil {
		if errors.Is(err, jaws.ErrValueNotFinite) {
			elem.Cancel(err)
			return nil
		}
		return err
	}
	attrs := append(elem.ApplyParams(params), getterAttrs...)
	if !u.binding.writable() {
		attrs = append(attrs, template.HTMLAttr("disabled"))
	}
	if err = htmlio.WriteHTMLInput(w, elem.Jid(), "range", text, attrs); err == nil {
		u.Last.Store(value)
		u.lastValid.Store(true)
	}
	return
}

// JawsUpdate updates the range when its exact source value changes.
func (u *Range) JawsUpdate(elem *jaws.Element) {
	value, text, err := u.binding.get(elem)
	if err != nil {
		elem.Cancel(err)
		return
	}
	prev := u.Last.Swap(value)
	wasValid := u.lastValid.Swap(true)
	if !wasValid || prev == nil || !u.binding.equal(prev, value) {
		elem.SetValue(text)
	}
}

// JawsInput stores a browser-side range value.
//
// Fractional integer input and values outside the source type's range are rejected
// without calling the setter or returning an error. For an ordinarily rendered
// Range, rejection dirties the source tag so the canonical source value is
// restored. Getter-only Ranges ignore browser input.
func (u *Range) JawsInput(elem *jaws.Element, text string) (err error) {
	if !u.binding.writable() {
		return
	}
	value, ok, parseErr := u.binding.parse(text)
	if parseErr != nil {
		elem.Cancel(parseErr)
		return
	}
	if !ok {
		u.lastValid.Store(false)
		elem.Dirty(u.tag)
		return
	}
	u.Last.Store(value)
	u.lastValid.Store(true)
	err = u.maybeDirty(elem, u.binding.set(elem, value))
	return
}

// Range renders an HTML range input for value.
//
// Numeric static values and Getter sources render disabled. Setter sources are
// editable. Supply min, max, and step attributes in params when the browser
// defaults do not cover the source's domain. For example:
//
//	rw.Range(floatBinder, `min="0"`, `max="200"`, `step="0.5"`)
func (rw RequestWriter) Range(value any, params ...any) error {
	if _, custom := value.(*NumericBinding); custom {
		panic(fmt.Errorf("expected numeric value or numeric bind.Getter, not %T", value))
	}
	binding, ok := makeNumericBinding(value)
	if !ok {
		panic(fmt.Errorf("expected numeric value or numeric bind.Getter, not %T", value))
	}
	return rw.NewUI(newRange(binding), params...)
}
