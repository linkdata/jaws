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
	JawsInput(e *Element, val string) (err error)
}

type errEventUnhandled struct{}

func (errEventUnhandled) Error() string {
	return "event unhandled"
}

// ErrEventUnhandled returned by JawsInput(), JawsClick() or
// JawsContextMenu() causes the next available handler to be invoked.
var ErrEventUnhandled = errEventUnhandled{}

// InputFn is the signature of an input handling function to be called when JaWS receives
// an input, hook or set message from Javascript via the WebSocket connection.
type InputFn = func(e *Element, val string) (err error)

func callInputHandler(obj any, e *Element, val string) (err error) {
	if h, ok := obj.(InputHandler); ok {
		return h.JawsInput(e, val)
	}
	if fn, ok := obj.(InputFn); ok {
		return fn(e, val)
	}
	return ErrEventUnhandled
}

func callEventHandler(obj any, e *Element, wht what.What, val string) (err error) {
	err = ErrEventUnhandled
	switch wht {
	case what.Click, what.ContextMenu:
		var clk Click
		var ok bool
		if clk, _, ok = parseClickData(val); ok {
			if wht == what.Click {
				if h, ok := obj.(ClickHandler); ok {
					err = h.JawsClick(e, clk)
				}
			} else if h, ok := obj.(ContextMenuHandler); ok {
				err = h.JawsContextMenu(e, clk)
			}
		}
	case what.Input, what.Hook, what.Set:
		err = callInputHandler(obj, e, val)
	}
	return
}

func callEventHandlers(ui any, e *Element, wht what.What, val string) (err error) {
	for i := len(e.handlers) - 1; i >= 0; i-- {
		if err = callEventHandler(e.handlers[i], e, wht, val); !errors.Is(err, ErrEventUnhandled) {
			return
		}
	}
	return callEventHandler(ui, e, wht, val)
}

// CallEventHandlers calls the event handlers for the given Element.
// Recovers from panics in user-provided handlers, returning them as errors.
func CallEventHandlers(ui any, e *Element, wht what.What, val string) (err error) {
	defer func() {
		if x := recover(); x != nil {
			err = errEventHandlerPanic{
				Type:  reflect.TypeOf(ui),
				Value: x,
			}
		}
	}()
	return callEventHandlers(ui, e, wht, val)
}
