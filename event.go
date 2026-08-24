package jaws

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/linkdata/jaws/lib/what"
)

// Click identifies a browser click-like event, pointer location and modifier state.
type Click struct {
	// Name is the event target name. Parsing off the wire normalizes it: leading
	// and trailing whitespace is trimmed and internal whitespace runs collapse to a
	// single space. [Click.String] applies the same normalization when formatting.
	Name    string
	X       float64 // X is the browser clientX coordinate in CSS pixels.
	Y       float64 // Y is the browser clientY coordinate in CSS pixels.
	Shift   bool    // Shift reports whether the Shift key was held during the event.
	Control bool    // Control reports whether the Control key was held during the event.
	Alt     bool    // Alt reports whether the Alt key was held during the event.
}

const (
	clickKeyShift = (1 << iota)
	clickKeyControl
	clickKeyAlt
	clickKeyMask = clickKeyShift | clickKeyControl | clickKeyAlt
)

func (clk Click) keyState() (state int) {
	if clk.Shift {
		state |= clickKeyShift
	}
	if clk.Control {
		state |= clickKeyControl
	}
	if clk.Alt {
		state |= clickKeyAlt
	}
	return
}

// setKeyState sets the Shift, Control and Alt fields from the bitmask produced by
// keyState, used when parsing an incoming click off the wire.
func (clk *Click) setKeyState(state int) {
	clk.Shift = (state & clickKeyShift) != 0
	clk.Control = (state & clickKeyControl) != 0
	clk.Alt = (state & clickKeyAlt) != 0
}

// String formats clk for the JaWS wire protocol.
//
// It normalizes leading, trailing and repeated internal whitespace in
// [Click.Name] to the wire representation described by the Name field.
func (clk Click) String() string {
	name := strings.Join(strings.Fields(clk.Name), " ")
	return fmt.Sprintf("%s %s %d %s", runFormatFloat(clk.X), runFormatFloat(clk.Y), clk.keyState(), name)
}

func parseClickData(value string) (clk Click, after string, ok bool) {
	var clickPart string
	clickPart, after, _ = strings.Cut(value, "\t")
	var n int
	var kstate int
	var name strings.Builder
	ok = true
	for field := range strings.FieldsSeq(clickPart) {
		if ok {
			switch n {
			case 0:
				clk.X, ok = runAtof(field)
			case 1:
				clk.Y, ok = runAtof(field)
			case 2:
				kstate, ok = runAtoi(field)
				if ok {
					ok = kstate >= 0 && kstate&^clickKeyMask == 0
				}
				if ok {
					clk.setKeyState(kstate)
				}
			case 3:
				// First name token: assign directly so the common single-token click
				// stays allocation-free.
				clk.Name = field
			default:
				// Second or later name token: accumulate with a strings.Builder, seeded
				// once with the first token and joined by single spaces, so the name stays
				// O(total length) rather than the O(n^2) of naive per-token string
				// concatenation. The click frame is untrusted browser input sized up to the
				// WebSocket read limit.
				if name.Len() == 0 {
					name.Grow(len(clk.Name) + 1 + len(field))
					name.WriteString(clk.Name)
				}
				name.WriteByte(' ')
				name.WriteString(field)
			}
			n++
		}
	}
	if name.Len() > 0 {
		clk.Name = name.String()
	}
	ok = ok && n >= 3
	return
}

func runFormatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func runAtof(value string) (n float64, ok bool) {
	var err error
	n, err = strconv.ParseFloat(value, 64)
	// A range error (e.g. "1e999") still yields a usable ±Inf; only a syntax error is
	// a malformed frame. Accept the value either way so the event dispatch's finiteness
	// check can terminate the Request on a non-finite coordinate.
	ok = err == nil || errors.Is(err, strconv.ErrRange)
	return
}

// finite reports whether f is neither NaN nor infinite.
func finite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

func runAtoi(value string) (n int, ok bool) {
	var err error
	n, err = strconv.Atoi(value)
	ok = err == nil
	return
}

// InputHandler handles input-like messages for an Element.
type InputHandler interface {
	// JawsInput is called when JaWS dispatches an input-like message for an
	// [Element]. See [InputFn] for the message kinds.
	//
	// The bundled client sends input and set messages only while its WebSocket is
	// open and does not queue them for later delivery. Native changes that emit
	// neither input nor change do not invoke JawsInput.
	JawsInput(elem *Element, value string) (err error)
}

// InputFn is the signature of an input handling function.
//
// JaWS calls it for an input or set message received from JavaScript over the
// WebSocket connection, and for a hook message, which tests use to invoke the
// handler synchronously (see [what.Hook]).
//
// When a function value is used directly as an input handler through an
// any-valued API such as [ParseParams], [Element.AddHandlers] or
// [CallEventHandlers], its dynamic type must be exactly InputFn. Convert a value
// of a defined function type to InputFn first, or implement [InputHandler].
type InputFn = func(elem *Element, value string) (err error)

func callInputHandler(obj any, elem *Element, value string) (err error) {
	if h, ok := obj.(InputHandler); ok {
		return h.JawsInput(elem, value)
	}
	if fn, ok := obj.(InputFn); ok {
		return fn(elem, value)
	}
	return ErrEventUnhandled
}

func callEventHandler(obj any, elem *Element, wht what.What, value string) (err error) {
	err = ErrEventUnhandled
	switch wht {
	case what.Click, what.ContextMenu:
		var clk Click
		var ok bool
		if clk, _, ok = parseClickData(value); ok {
			if !finite(clk.X) || !finite(clk.Y) {
				// A non-finite coordinate cannot come from a well-behaved browser;
				// terminate the Request rather than dispatch a garbage click. Report the
				// event handled (nil) so the dispatch loop stops without also alerting a
				// connection that is being torn down.
				elem.Request.Cancel(fmt.Errorf("%w: click %v,%v", ErrValueNotFinite, clk.X, clk.Y))
				err = nil
				return
			}
			if wht == what.Click {
				if h, ok := obj.(ClickHandler); ok {
					err = h.JawsClick(elem, clk)
				}
			} else if h, ok := obj.(ContextMenuHandler); ok {
				err = h.JawsContextMenu(elem, clk)
			}
		}
	case what.Input, what.Hook, what.Set:
		err = callInputHandler(obj, elem, value)
	}
	return
}

func callEventHandlers(ui any, elem *Element, wht what.What, value string) (err error) {
	for i := len(elem.handlers) - 1; i >= 0; i-- {
		if err = callEventHandler(elem.handlers[i], elem, wht, value); !errors.Is(err, ErrEventUnhandled) {
			return
		}
	}
	return callEventHandler(ui, elem, wht, value)
}

// CallEventHandlers calls the event handlers for the given [Element].
//
// Recovers from panics in user-provided handlers, returning them as errors.
// Input callback functions used directly by signature are recognized according
// to the dynamic-type rules documented by [InputFn].
//
// It must not run concurrently with rendering or handler registration.
func CallEventHandlers(ui any, elem *Element, wht what.What, value string) (err error) {
	defer func() {
		if x := recover(); x != nil {
			err = errEventHandlerPanic{
				Type:  reflect.TypeOf(ui),
				Value: x,
			}
		}
	}()
	return callEventHandlers(ui, elem, wht, value)
}
