package jaws

import (
	"fmt"
)

type ErrPendingCancelled struct {
	JawsKey uint64
	Cause   error
	Initial string
}

func (e ErrPendingCancelled) Error() string {
	return fmt.Sprintf("Request<%s>:%s %v", JawsKeyString(e.JawsKey), e.Initial, e.Cause)
}

func (e ErrPendingCancelled) Is(target error) (yes bool) {
	_, yes = target.(ErrPendingCancelled)
	return
}

func (e ErrPendingCancelled) Unwrap() error {
	return e.Cause
}

func newErrPendingCancelled(rq *Request, cause error) (err error) {
	var initial string
	if rq.Initial != nil {
		initial = fmt.Sprintf(" %s %q:", rq.Initial.Method, rq.Initial.RequestURI)
	}
	return ErrPendingCancelled{
		JawsKey: rq.JawsKey,
		Cause:   cause,
		Initial: initial,
	}
}
