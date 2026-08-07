package ui

import (
	"strconv"
)

// ErrElementStateUnclaimed reports an update of an Element that no [Template]
// rendered.
//
// [Template.JawsUpdate] reports this error through [jaws.Request.MustLog] instead
// of returning it.
var ErrElementStateUnclaimed errElementStateUnclaimed

type errElementStateUnclaimed string

func (e errElementStateUnclaimed) Error() string {
	return "template " + strconv.Quote(string(e)) + " updating an element it did not render"
}

func (errElementStateUnclaimed) Is(target error) bool {
	return target == ErrElementStateUnclaimed
}
