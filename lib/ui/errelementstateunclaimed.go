package ui

import (
	"strconv"
)

// ErrElementStateUnclaimed reports a wrapped [Template] updating an [jaws.Element] no
// Template claimed while rendering.
//
// With no claim there is no previous generation to reconcile against, so the update
// executes nothing. The usual cause is updating an Element that was never rendered, such
// as a wrapped Template used as a [RequestWriter.Register] updater.
//
// [Template.JawsUpdate] reports it through [jaws.Request.MustLog] rather than returning
// it, so it reaches a caller as a returned error only where that panic is recovered: an
// `html/template` action wraps it in an execution error, which [errors.Is] resolves
// through. Template lookup happens first, so a missing template reports
// [ErrMissingTemplate] and this error is reached only after a successful lookup.
var ErrElementStateUnclaimed errElementStateUnclaimed

type errElementStateUnclaimed string

func (e errElementStateUnclaimed) Error() string {
	return "template " + strconv.Quote(string(e)) + " updating an element it did not render"
}

func (errElementStateUnclaimed) Is(target error) bool {
	return target == ErrElementStateUnclaimed
}
