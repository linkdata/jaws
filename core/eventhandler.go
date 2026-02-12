package core

import (
	"github.com/linkdata/jaws/what"
)

type EventHandler interface {
	JawsEvent(e *Element, wht what.What, val string) (err error)
}

type errEventUnhandled struct{}

func (errEventUnhandled) Error() string {
	return "event unhandled"
}

// ErrEventUnhandled returned by JawsEvent() or JawsClick() causes the next
// available handler to be invoked.
var ErrEventUnhandled = errEventUnhandled{}

// EventFn is the signature of a event handling function to be called when JaWS receives
// an event message from the Javascript via the WebSocket connection.
type EventFn = func(e *Element, wht what.What, val string) (err error)

type eventFnWrapper struct{ EventFn }

func (ehf eventFnWrapper) JawsEvent(e *Element, w what.What, v string) (err error) {
	return ehf.EventFn(e, w, v)
}

var _ EventFn = eventFnWrapper{}.JawsEvent // statically ensure JawsEvent and EventFn are compatible

func callEventHandler(obj any, e *Element, wht what.What, val string) (err error) {
	if wht == what.Click {
		if h, ok := obj.(ClickHandler); ok {
			if err = h.JawsClick(e, val); err != ErrEventUnhandled {
				return
			}
		}
	}
	if h, ok := obj.(EventHandler); ok {
		return h.JawsEvent(e, wht, val)
	}
	return ErrEventUnhandled
}

func CallEventHandlers(ui any, e *Element, wht what.What, val string) (err error) {
	if err = callEventHandler(ui, e, wht, val); err == ErrEventUnhandled {
		for _, h := range e.handlers {
			if err = callEventHandler(h, e, wht, val); err != ErrEventUnhandled {
				return
			}
		}
	}
	return
}
