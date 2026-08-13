package jaws

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/linkdata/jaws/lib/what"
)

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
// Request event dispatch calls this only after the Element is frozen, publishing
// the completed handler slice before its lock-free read. A direct caller must not
// run it concurrently with rendering or handler registration.
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
