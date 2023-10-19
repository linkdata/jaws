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

	tests := []struct {
		name string
		v    interface{}
		want TimeSetter
		out  time.Time
		tag  interface{}
	}{
		{
			name: "time.Time",
			v:    val,
			want: timeGetter{val},
			out:  val,
			tag:  nil,
		},
		{
			name: "timeGetter",
			v:    timeGetter{val},
			want: timeGetter{val},
			out:  val,
			tag:  nil,
		},
		{
			name: "*atomic.Value",
			v:    &av,
			want: atomicGetter{&av},
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
			if tag := got.(TagGetter).JawsGetTag(nil); tag != tt.tag {
				t.Errorf("makeTimeSetter().JawsGetTag() = %v, want %v", tag, tt.tag)
			}
		})
	}
}
