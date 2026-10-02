package ui

import (
	"strconv"
)

// ErrElementStateUnclaimed reports an update without rendered widget state.
//
// [Template.JawsUpdate], [Container.JawsUpdate], and [Select.JawsUpdate]
// report this error through [jaws.Request.MustLog] instead of returning it.
var ErrElementStateUnclaimed errElementStateUnclaimed

type errElementStateUnclaimed string

func (e errElementStateUnclaimed) Error() string {
	if e == "" {
		return "element state unclaimed"
	}
	return "template " + strconv.Quote(string(e)) + " updating an element it did not render"
}

func (errElementStateUnclaimed) Is(target error) bool {
	return target == ErrElementStateUnclaimed
}
