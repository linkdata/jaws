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
// Number requires ordinary rendering and is not supported by
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
// input is editable and source must expose the stable usable tag described by
// [Input]. A getter-only source renders read-only.
//
// A non-finite bound floating-point value cancels the [jaws.Request] with a cause
// matching [jaws.ErrValueNotFinite].
func NewNumber[T Numeric](source bind.Getter[T]) *Number {
	return newNumber(newBuiltinNumericBinding(source))
}

// NewNumberWith returns a number input widget using codec.
//
// The source capability and tag rules are the same as for [NewNumber]. Browser
// text must use the HTML valid-floating-point grammar before it is passed to
// [NumberCodec.ParseNumber]. The codec must be concurrency-safe and provide a
// deterministic round trip for every source value. A codec or value contract
// violation is reported with [ErrNumberFormat] during rendering and cancels the
// Request during input or update. T must be strictly comparable, and its bound and
// parsed values must equal themselves.
func NewNumberWith[T comparable](source bind.Getter[T], codec NumberCodec[T]) *Number {
	return newNumber(newCustomNumericBinding(source, codec))
}

func newNumber(binding *numericBinding) *Number {
	return &Number{binding: binding}
}

// JawsRender renders ui as an HTML number input.
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

// JawsUpdate sends the canonical source value when it differs from the browser
// baseline.
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
// Numeric static values and Getter sources render read-only. Setter sources are
// editable. Pass a [NumericBinding] to use a custom [NumberCodec] from a template.
func (rw RequestWriter) Number(value any, params ...any) error {
	binding, ok := makeNumericBinding(value)
	if !ok {
		panic(fmt.Errorf("expected numeric value or numeric bind.Getter, not %T", value))
	}
	return rw.NewUI(newNumber(binding), params...)
}
