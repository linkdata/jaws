package jaws

import (
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func Test_makeTimeGetter_panic(t *testing.T) {
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
	makeTimeGetter(uint32(42))
}

func Test_makeTimeGetter(t *testing.T) {
	val := time.Now()
	var av atomic.Value
	av.Store(val)

	tests := []struct {
		name string
		v    interface{}
		want TimeGetter
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
			got := makeTimeGetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeTimeGetter() = %v, want %v", got, tt.want)
			}
			if out := got.JawsGetTime(nil); out != tt.out {
				t.Errorf("makeTimeGetter().JawsGetTime() = %v, want %v", out, tt.out)
			}
			if tag := got.(TagGetter).JawsGetTag(nil); tag != tt.tag {
				t.Errorf("makeTimeGetter().JawsGetTag() = %v, want %v", tag, tt.tag)
			}
		})
	}
}
