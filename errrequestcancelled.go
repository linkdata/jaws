package jaws

import "fmt"

// ErrRequestCancelled indicates a Request was cancelled. Use Unwrap() to see the underlying cause.
var ErrRequestCancelled errRequestCancelled

type errRequestCancelled struct {
	JawsKey Key
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
