package jaws

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/linkdata/jaws/lib/what"
)

// InputHandler handles input events sent from the browser.
type InputHandler interface {
	// JawsInput is called when an [Element] receives a browser input event.
	JawsInput(elem *Element, value string) (err error)
}

// InputFn is the signature of an input handling function. JaWS calls it for an
// input or set message received from JavaScript over the WebSocket connection,
// and for a hook message, which tests use to invoke the handler synchronously
// (see [what.Hook]).
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
