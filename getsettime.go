package jaws

import (
	"sync/atomic"
	"time"
)

type TimeGetter interface {
	JawsGetTime(rq *Request) time.Time
}

type TimeSetter interface {
	TimeGetter
	JawsSetTime(rq *Request, v time.Time) (err error)
}

type timeGetter struct{ v time.Time }

func (g timeGetter) JawsGetTime(rq *Request) time.Time {
	return g.v
}

func (g timeGetter) JawsGetTag(rq *Request) interface{} {
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
