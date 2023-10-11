package jaws

import "github.com/linkdata/jaws/what"

type EventHandler interface {
	JawsEvent(e *Element, wht what.What, val string) (stop bool, err error)
}

// EventFn is the signature of a event handling function to be called when JaWS receives
// an event message from the Javascript via the WebSocket connection.
type EventFn = func(e *Element, wht what.What, val string) (stop bool, err error)

type eventFnWrapper struct{ EventFn }

func (ehf eventFnWrapper) JawsEvent(e *Element, w what.What, v string) (stop bool, err error) {
	return ehf.EventFn(e, w, v)
}

var _ EventFn = eventFnWrapper{}.JawsEvent // statically ensure JawsEvent and EventFn are compatible

func callEventHandler(obj any, e *Element, wht what.What, val string) (stop bool, err error) {
	if wht == what.Click {
		if h, ok := obj.(ClickHandler); ok {
			if stop, err = h.JawsClick(e, val); stop || err != nil {
				return
			}
		}
	}
	if h, ok := obj.(EventHandler); ok {
		return h.JawsEvent(e, wht, val)
	}
	return
}

func callAllEventHandlers(e *Element, wht what.What, val string) error {
	if stop, err := callEventHandler(e.ui, e, wht, val); stop || err != nil {
		return err
	}
	for _, h := range e.handlers {
		if stop, err := h.JawsEvent(e, wht, val); stop || err != nil {
			return err
		}
	}
	return nil
}
