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
	_, yes = target.(errPendingCancelled)
	return
}

func (e errPendingCancelled) Unwrap() error {
	return e.Cause
}

func newErrPendingCancelled(rq *Request, cause error) (err error) {
	var initial string
	if rq.Initial != nil {
		initial = fmt.Sprintf(" %s %q:", rq.Initial.Method, rq.Initial.RequestURI)
	}
	return errPendingCancelled{
		JawsKey: rq.JawsKey,
		Cause:   cause,
		Initial: initial,
	}
}
