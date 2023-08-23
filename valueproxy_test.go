package jaws

import (
	"reflect"
	"sync/atomic"
	"testing"
)

type testValueProxy struct {
	v         interface{}
	makeErr   error
	getCalled bool
	setCalled bool
}

func (vp *testValueProxy) JawsGet(e *Element) interface{} {
	vp.getCalled = true
	return vp.v
}

func (vp *testValueProxy) JawsSet(e *Element, val interface{}) (err error) {
	vp.v = val
	vp.setCalled = true
	return vp.makeErr
}

func TestMakeValueProxy(t *testing.T) {
	type args struct {
		value interface{}
	}

	var av atomic.Value
	av.Store(12345)

	tests := []struct {
		name   string
		args   args
		wantVp ValueProxy
	}{
		{
			name:   "untyped nil",
			args:   args{nil},
			wantVp: &defaultValueProxy{},
		},
		{
			name:   "string",
			args:   args{"meh"},
			wantVp: &defaultValueProxy{v: "meh"},
		},
		{
			name:   "testValueProxy",
			args:   args{&testValueProxy{v: 1234}},
			wantVp: &testValueProxy{v: 1234},
		},
		{
			name:   "*atomic.Value",
			args:   args{&av},
			wantVp: atomicValueProxy{&av},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotVp := MakeValueProxy(tt.args.value); !reflect.DeepEqual(gotVp, tt.wantVp) {
				t.Errorf("MakeValueProxy() = %v, want %v", gotVp, tt.wantVp)
			}
		})
	}
}
