package jaws

import (
	"fmt"
)

// ErrPendingCancelled indicates a pending Request was cancelled. Use Unwrap() to see the underlying cause.
var ErrPendingCancelled errPendingCancelled

type errPendingCancelled struct {
	JawsKey uint64
	Cause   error
	Initial string
}

func (e errPendingCancelled) Error() string {
	return fmt.Sprintf("Request<%s>:%s %v", JawsKeyString(e.JawsKey), e.Initial, e.Cause)
}

func (e errPendingCancelled) Is(target error) (yes bool) {
	return target == ErrPendingCancelled
}

func (e errPendingCancelled) Unwrap() error {
	return e.Cause
}

func newErrPendingCancelledLocked(rq *Request, cause error) (err error) {
	var initial string
	if rq.initial != nil {
		initial = fmt.Sprintf(" %s %q:", rq.initial.Method, rq.initial.RequestURI)
	}
	return errPendingCancelled{
		JawsKey: rq.JawsKey,
		Cause:   cause,
		Initial: initial,
	}
}
