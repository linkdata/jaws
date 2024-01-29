package jaws

import (
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var _ TimeSetter = (*testSetter[time.Time])(nil)

func Test_makeTimeSetter_panic(t *testing.T) {
	defer func() {
		if x := recover(); x != nil {
			if err, ok := x.(error); ok {
				if strings.Contains(err.Error(), "uint32") {
					return
				}
			}
		}
		t.Fail()
	}()
	makeTimeSetter(uint32(42))
}

func Test_makeTimeSetter(t *testing.T) {
	val := time.Now()
	var av atomic.Value
	av.Store(val)
	ts := newTestSetter(val)

	tests := []struct {
		name string
		v    any
		want TimeSetter
		out  time.Time
		err  error
		tag  any
	}{
		{
			name: "TimeSetter",
			v:    ts,
			want: ts,
			out:  val,
			tag:  ts,
		},
		{
			name: "time.Time",
			v:    val,
			want: timeGetter{val},
			out:  val,
			err:  ErrValueNotSettable,
			tag:  nil,
		},
		{
			name: "*atomic.Value",
			v:    &av,
			want: atomicSetter{&av},
			out:  val,
			tag:  &av,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeTimeSetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeTimeSetter() = %v, want %v", got, tt.want)
			}
			if out := got.JawsGetTime(nil); out != tt.out {
				t.Errorf("makeTimeSetter().JawsGetTime() = %v, want %v", out, tt.out)
			}
			if err := got.JawsSetTime(nil, val.Add(time.Minute)); err != tt.err {
				t.Errorf("makeTimeSetter().JawsSetTime() = %v, want %v", err, tt.err)
			}
			gotTag := any(got)
			if tg, ok := got.(TagGetter); ok {
				gotTag = tg.JawsGetTag(nil)
			}
			if gotTag != tt.tag {
				t.Errorf("makeTimeSetter().tag = %v, want %v", gotTag, tt.tag)
			}
		})
	}
}
