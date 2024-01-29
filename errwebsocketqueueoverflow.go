package jaws

import (
	"fmt"
)

// ErrWebsocketQueueOverflow is returned when an Element queues up too many changes.
// Try reducing the number of JawsUpdate() calls to the element or the number of
// changes made during JawsUpdate() for the UI element.
var ErrWebsocketQueueOverflow errWebsocketQueueOverflow

type errWebsocketQueueOverflow struct {
	str string
}

func (e errWebsocketQueueOverflow) Error() string {
	return fmt.Sprintf("WebSocket queue overflow on %s", e.str)
}

func (e errWebsocketQueueOverflow) Is(target error) (yes bool) {
	_, yes = target.(errWebsocketQueueOverflow)
	return
}

func newErrWebsocketQueueOverflow(e *Element) error {
	return errWebsocketQueueOverflow{str: e.String()}
}
