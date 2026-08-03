package ui

import (
	"strconv"
)

// ErrElementStateUnclaimed is returned when a wrapped [Template] is asked to update a
// [jaws.Element] that no Template claimed while rendering, so there is no previous
// generation to reconcile against.
//
// The usual cause is updating an Element that was never rendered, such as a wrapped
// Template used as a [RequestWriter.Register] updater.
var ErrElementStateUnclaimed errElementStateUnclaimed

type errElementStateUnclaimed string

func (e errElementStateUnclaimed) Error() string {
	return "template " + strconv.Quote(string(e)) + " updating an element it did not render"
}

func (errElementStateUnclaimed) Is(target error) bool {
	return target == ErrElementStateUnclaimed
}
