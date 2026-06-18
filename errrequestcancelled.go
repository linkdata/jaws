package jaws

import (
	"fmt"

	"github.com/linkdata/jaws/lib/key"
)

// ErrRequestCancelled indicates a [Request] was cancelled.
//
// The concrete error reachable via [context.Cause] on [Request.Context] wraps the
// underlying cancellation cause, so it can be matched with [errors.Is] and its cause
// retrieved with Unwrap. The exported sentinel itself carries no cause.
var ErrRequestCancelled errRequestCancelled

type errRequestCancelled struct {
	JawsKey key.Key
	Cause   error
	Initial string
}

func (e errRequestCancelled) Error() string {
	return fmt.Sprintf("Request<%s>:%s %v", e.JawsKey, e.Initial, e.Cause)
}

func (e errRequestCancelled) Is(target error) (yes bool) {
	return target == ErrRequestCancelled
}

func (e errRequestCancelled) Unwrap() error {
	return e.Cause
}

func newErrRequestCancelledLocked(rq *Request, cause error) (err error) {
	if cause != nil {
		var initial string
		if rq.initial != nil {
			initial = fmt.Sprintf(" %s %q:", rq.initial.Method, rq.initial.RequestURI)
		}
		err = errRequestCancelled{
			JawsKey: rq.JawsKey,
			Cause:   cause,
			Initial: initial,
		}
	}
	return
}
