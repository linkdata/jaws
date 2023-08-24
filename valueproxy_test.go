package jaws

import (
	"reflect"
	"sync/atomic"
	"testing"
)

type testValueProxy struct {
	v         interface{}
	getCalled bool
	setCalled bool
}

func (vp *testValueProxy) JawsGet(e *Element) interface{} {
	vp.getCalled = true
	return vp.v
}

func (vp *testValueProxy) JawsSet(e *Element, val interface{}) (changed bool) {
	changed = vp.v != val
	vp.v = val
	vp.setCalled = true
	return
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
			name:   "testValueProxy",
			args:   args{&testValueProxy{v: 1234}},
			wantVp: &testValueProxy{v: 1234},
		},
		{
			name:   "*atomic.Value",
			args:   args{&av},
			wantVp: AtomicValueProxy{&av},
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

func TestMakeValueProxy_PanicsOnValue(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fail()
		}
	}()
	MakeValueProxy(1)
}

func TestMakeValueProxy_PanicsOnAtomic(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fail()
		}
	}()
	var av atomic.Value
	MakeValueProxy(av)
}
