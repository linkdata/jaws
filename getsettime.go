package jaws

import (
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

func (g timeGetter) JawsGetTag(e *Element) interface{} {
	return nil
}

func makeTimeGetter(v interface{}) TimeGetter {
	switch v := v.(type) {
	case TimeGetter:
		return v
	case time.Time:
		return timeGetter{v}
	case *atomic.Value:
		return atomicGetter{v}
	}
	panic("makeTimeGetter: invalid type")
}
