package jaws

import (
	"fmt"
	"sync/atomic"
	"time"
)

type TimeSetter interface {
	JawsGetTime(e *Element) time.Time
	// JawsSetTime may return ErrValueUnchanged to indicate value was already set.
	JawsSetTime(e *Element, v time.Time) (err error)
}

type timeGetter struct{ v time.Time }

func (g timeGetter) JawsGetTime(e *Element) time.Time {
	return g.v
}

func (g timeGetter) JawsSetTime(*Element, time.Time) error {
	return ErrValueNotSettable
}

func (g timeGetter) JawsGetTag(rq *Request) any {
	return nil
}

func makeTimeSetter(v any) TimeSetter {
	switch v := v.(type) {
	case TimeSetter:
		return v
	case time.Time:
		return timeGetter{v}
	case *atomic.Value:
		return atomicSetter{v}
	}
	panic(fmt.Errorf("expected jaws.TimeGetter or time.Time, not %T", v))
}
