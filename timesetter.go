package jaws

import (
	"fmt"
	"sync/atomic"
	"time"
)

type TimeSetter interface {
	JawsGetTime(e *Element) time.Time
	JawsSetTime(e *Element, v time.Time) (err error)
}

type timeGetter struct{ v time.Time }

func (g timeGetter) JawsGetTime(e *Element) time.Time {
	return g.v
}

func (g timeGetter) JawsSetTime(*Element, time.Time) error {
	return ErrValueNotSettable
}

func (g timeGetter) JawsGetTag(rq *Request) interface{} {
	return nil
}

func makeTimeSetter(v interface{}) TimeSetter {
	switch v := v.(type) {
	case TimeSetter:
		return v
	case time.Time:
		return timeGetter{v}
	case *atomic.Value:
		return atomicGetter{v}
	}
	panic(fmt.Errorf("expected jaws.TimeGetter or time.Time, not %T", v))
}
