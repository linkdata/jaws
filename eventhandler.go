package jaws

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/linkdata/jaws/lib/what"
)

// ErrEventHandlerPanic is returned when an event handler panics.
var ErrEventHandlerPanic errEventHandlerPanic

type errEventHandlerPanic struct {
	Type  reflect.Type
	Value any
}

func (e errEventHandlerPanic) Error() string {
	return fmt.Sprintf("jaws: %v panic: %v", e.Type, e.Value)
}

func (errEventHandlerPanic) Is(target error) bool {
	return target == ErrEventHandlerPanic
}

func (e errEventHandlerPanic) Unwrap() error {
	if err, ok := e.Value.(error); ok {
		return err
	}
	return nil
}

// InputHandler handles input events sent from the browser.
type InputHandler interface {
	// JawsInput is called when an [Element] receives a browser input event.
	JawsInput(elem *Element, value string) (err error)
}

type errEventUnhandled struct{}

func (errEventUnhandled) Error() string {
	return "event unhandled"
}

// ErrEventUnhandled returned by [InputHandler.JawsInput], [ClickHandler.JawsClick]
// or [ContextMenuHandler.JawsContextMenu] causes the next available handler to be invoked.
var ErrEventUnhandled = errEventUnhandled{}

// InputFn is the signature of an input handling function to be called when JaWS receives
// an input, hook or set message from JavaScript via the WebSocket connection.
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
// Recovers from panics in user-provided handlers, returning them as errors.
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
