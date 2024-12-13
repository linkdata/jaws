package jaws

import (
	"reflect"
	"sync/atomic"
	"testing"
)

var _ AnySetter = (*testSetter[any])(nil)

func Test_makeAnySetter(t *testing.T) {
	val := float64(12.34)
	var av atomic.Value
	av.Store(val)
	ts := newTestSetter(any(val))

	tests := []struct {
		name string
		v    any
		want AnySetter
		in   any
		out  any
		err  error
		tag  any
	}{
		{
			name: "AnySetter",
			v:    ts,
			want: ts,
			out:  val,
			tag:  ts,
		},
		{
			name: "read-only",
			v:    val,
			want: anyGetter{val},
			in:   -val,
			out:  val,
			err:  ErrValueNotSettable,
			tag:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeAnySetter(tt.v)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeAnySetter() = %v, want %v", got, tt.want)
			}
			if out := got.JawsGetAny(nil); out != tt.out {
				t.Errorf("makeAnySetter().JawsGetAny() = %v, want %v", out, tt.out)
			}
			if tt.in != nil {
				if err := got.JawsSetAny(nil, tt.in); err != tt.err {
					t.Errorf("makeAnySetter().JawsSetAny() = %v, want %v", err, tt.err)
				}
			}
			gotTag := any(got)
			if tg, ok := got.(TagGetter); ok {
				gotTag = tg.JawsGetTag(nil)
			}
			if gotTag != tt.tag {
				t.Errorf("makeAnySetter().tag = %v, want %v", gotTag, tt.tag)
			}

		})
	}
}
