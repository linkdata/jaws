package jaws

import "github.com/linkdata/jaws/what"

type EventHandler interface {
	JawsEvent(e *Element, wht what.What, val string) (err error)
}

// EventFn is the signature of a event handling function to be called when JaWS receives
// an event message from the Javascript via the WebSocket connection.
type EventFn = func(e *Element, wht what.What, val string) error

type eventFnWrapper struct{ EventFn }

func (ehf eventFnWrapper) JawsEvent(e *Element, w what.What, v string) error {
	return ehf.EventFn(e, w, v)
}

var _ EventFn = eventFnWrapper{}.JawsEvent // statically ensure JawsEvent and EventFn are compatible

func callEventHandler(obj any, e *Element, wht what.What, val string) error {
	if wht == what.Click {
		if h, ok := obj.(ClickHandler); ok {
			if err := h.JawsClick(e, val); err != nil {
				return err
			}
		}
	}
	if h, ok := obj.(EventHandler); ok {
		if err := h.JawsEvent(e, wht, val); err != nil {
			return err
		}
	}
	return nil
}

func callAllEventHandlers(e *Element, wht what.What, val string) error {
	if err := callEventHandler(e.ui, e, wht, val); err != nil {
		return err
	}
	for _, h := range e.handlers {
		if err := h.JawsEvent(e, wht, val); err != nil {
			return err
		}
	}
	return nil
}
