package jaws

import (
	"fmt"
	"sync/atomic"
	"time"
)

type TimeGetter interface {
	JawsGetTime(e *Element) time.Time
}

type TimeSetter interface {
	TimeGetter
	JawsSetTime(e *Element, v time.Time) (err error)
}

type timeGetter struct{ v time.Time }

func (g timeGetter) JawsGetTime(e *Element) time.Time {
	return g.v
}

func (g timeGetter) JawsSetTime(e *Element, val time.Time) error {
	return nil
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
