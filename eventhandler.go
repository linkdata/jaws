package jaws

import (
	"errors"
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
// It reads the Element's handlers without a lock, relying on the render/freeze
// lifecycle (see the package "Locking" documentation): a direct caller must not
// invoke it concurrently with the Element's rendering or handler registration.
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
